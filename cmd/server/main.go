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
	"gopherledger/internal/config"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalln(err)
	}
	config.GlobalConfig = cfg
}
