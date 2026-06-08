package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type ParcelStatus string

const (
	Registered ParcelStatus = "registered"
	Sent       ParcelStatus = "sent"
	Delivered  ParcelStatus = "delivered"
)

type Parcel struct {
	Number    int
	Client    int
	Status    ParcelStatus
	Address   string
	CreatedAt string
}

type ParcelService struct {
	store *ParcelStore
}

func NewParcelService(store *ParcelStore) ParcelService {
	return ParcelService{store: store}
}

func (s ParcelService) Register(client int, address string) (Parcel, error) {
	if client <= 0 {
		return Parcel{}, errors.New("validation error: client ID must be > 0")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return Parcel{}, errors.New("validation error: address can not be empty")
	}
	if len(address) > 512 {
		return Parcel{}, errors.New("validation error: address exceeds max len = 512 characters")
	}

	parcel := Parcel{
		Client:    client,
		Status:    Registered,
		Address:   address,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	id, err := s.store.Add(parcel)
	if err != nil {
		return parcel, err
	}

	parcel.Number = id

	fmt.Printf("Новая посылка № %d на адрес %s от клиента с идентификатором %d зарегистрирована %s\n",
		parcel.Number, parcel.Address, parcel.Client, parcel.CreatedAt)

	return parcel, nil
}

func (s ParcelService) PrintClientParcels(client int) error {
	parcels, err := s.store.GetByClient(client)
	if err != nil {
		return err
	}

	fmt.Printf("Посылки клиента %d:\n", client)
	for _, parcel := range parcels {
		fmt.Printf("Посылка № %d на адрес %s от клиента с идентификатором %d зарегистрирована %s, статус '%s'\n",
			parcel.Number, parcel.Address, parcel.Client, parcel.CreatedAt, parcel.Status)
	}
	fmt.Println()

	return nil
}

func (s ParcelService) NextStatus(number int) error {
	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}
	parcel, err := s.store.Get(number)
	if err != nil {
		return err
	}

	var nextStatus ParcelStatus

	switch parcel.Status {
	case Registered:
		nextStatus = Sent
	case Sent:
		nextStatus = Delivered
	case Delivered:
		return nil
	}

	fmt.Printf("У посылки № %d новый статус: %s\n", number, nextStatus)

	return s.store.SetStatus(number, nextStatus)
}

func (s ParcelService) ChangeAddress(number int, address string) error {
	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}

	address = strings.TrimSpace(address)
	if address == "" {
		return errors.New("validation error: address can not be empty")
	}

	if len(address) > 512 {
		return errors.New("validation error: address exceeds max len = 512 characters")
	}

	return s.store.SetAddress(number, address)
}

func (s ParcelService) Delete(number int) error {
	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}
	return s.store.Delete(number)
}

func main() {
	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Printf("database is unavailable: %v\n", err)
		return
	}
	store := NewParcelStore(db)
	service := NewParcelService(store)

	// регистрация посылки
	client := 1
	address := "Псков, д. Пушкина, ул. Колотушкина, д. 5"
	p, err := service.Register(client, address)
	if err != nil {
		fmt.Println(err)
		return
	}

	// изменение адреса
	newAddress := "Саратов, д. Верхние Зори, ул. Козлова, д. 25"
	err = service.ChangeAddress(p.Number, newAddress)
	if err != nil {
		fmt.Println(err)
		return
	}

	// изменение статуса
	err = service.NextStatus(p.Number)
	if err != nil {
		fmt.Println(err)
		return
	}

	// вывод посылок клиента
	err = service.PrintClientParcels(client)
	if err != nil {
		fmt.Println(err)
		return
	}

	// попытка удаления отправленной посылки
	err = service.Delete(p.Number)
	if err != nil {
		fmt.Printf("[ОЖИДАЕМАЯ ОШИБКА - попытка удалить отправленную посылку]: %v\n\n", err)
		// удалила return, чтобы предотвратить преждевременное завершение сценария.
	}

	// вывод посылок клиента
	// предыдущая посылка не должна удалиться, т.к. её статус НЕ «зарегистрирована»
	err = service.PrintClientParcels(client)
	if err != nil {
		fmt.Println(err)
		return
	}

	// регистрация новой посылки
	p, err = service.Register(client, address)
	if err != nil {
		fmt.Println(err)
		return
	}

	// удаление новой посылки
	err = service.Delete(p.Number)
	if err != nil {
		fmt.Println(err)
		return
	}

	// вывод посылок клиента
	// здесь не должно быть последней посылки, т.к. она должна была успешно удалиться
	err = service.PrintClientParcels(client)
	if err != nil {
		fmt.Println(err)
		return
	}
}
