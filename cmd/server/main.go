// Точка входа сервера. Реализуйте самостоятельно.
//
// Порядок инициализации:
//  1. Загрузить конфигурацию (пакет config)
//  2. Создать хранилище (пакет store)
//  3. Создать сервис (пакет service)
//  4. Запустить воркер начислений в горутине (svc.StartAccrualWorker)
//  5. Создать обработчик и роутер (пакеты handler, router)
//  6. Запустить HTTP-сервер
//  7. Реализовать graceful shutdown по сигналам SIGINT и SIGTERM
package main

import (
	"context"
	"errors"
	"gopherledger/internal/config"
	"gopherledger/internal/handler"
	"gopherledger/internal/router"
	"gopherledger/internal/service"
	"gopherledger/internal/store"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalln(err)
	}
	config.GlobalConfig = cfg

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(config.GlobalConfig.DatabaseURI)
	if err != nil {
		log.Fatalf("ошибка конфигурации бд: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v", err)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("бд не отвечает после запуска: %v", err)
	}

	log.Print("успешное подключение к бд!")

	localStore := store.New(pool, config.GlobalConfig.DatabaseTimeout)
	localService := service.New(localStore)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		localService.StartAccrualWorker(
			ctx,
			config.GlobalConfig.AccrualIntervalSeconds,
			config.GlobalConfig.Workers,
		)
	}()

	h := handler.New(localService)
	mux := router.New(h)

	addr := "0.0.0.0:8080"
	server := &http.Server{
		Handler: mux,
		Addr:    addr,
	}

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	<-sigChan
	log.Println("Starting graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	cancel()

	wg.Wait()

	log.Printf("Server stopped")
}
