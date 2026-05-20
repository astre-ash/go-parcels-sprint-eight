package main

import (
	"database/sql"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	// randSource источник псевдо случайных чисел.
	// Для повышения уникальности в качестве seed
	// используется текущее время в unix формате (в виде числа)
	randSource = rand.NewSource(time.Now().UnixNano())
	// randRange использует randSource для генерации случайных чисел
	randRange = rand.New(randSource)
)

// getTestParcel возвращает тестовую посылку
func getTestParcel() Parcel {
	return Parcel{
		Client:    1000,
		Status:    Registered,
		Address:   "test",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func setupDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS parcel (
		number     INTEGER PRIMARY KEY AUTOINCREMENT,
		client     INTEGER NOT NULL,
		status     VARCHAR(128) NOT NULL,
		address    VARCHAR(512) NOT NULL,
		created_at TEXT NOT NULL);`)
	require.NoError(t, err)

	return db
}

// TestAddGetDelete проверяет добавление, получение и удаление посылки
func TestAddGetDelete(t *testing.T) {
	// arrange
	db := setupDB(t)
	defer db.Close()

	store := NewParcelStore(db)
	parcel := getTestParcel()

	// add
	number, err := store.Add(parcel)
	require.NoError(t, err)
	require.NotEmpty(t, number)

	parcel.Number = number

	// get
	res, err := store.Get(number)
	require.NoError(t, err)

	require.Equal(t, parcel.Number, res.Number)
	require.Equal(t, parcel.Client, res.Client)
	require.Equal(t, parcel.Status, res.Status)
	require.Equal(t, parcel.Address, res.Address)
	require.Equal(t, parcel.CreatedAt, res.CreatedAt)

	// delete
	err = store.Delete(number)
	require.NoError(t, err)

	_, err = store.Get(number)
	require.ErrorIs(t, err, sql.ErrNoRows)

}

// TestSetAddress проверяет обновление адреса
func TestSetAddress(t *testing.T) {
	// arrange
	db := setupDB(t)
	defer db.Close()

	store := NewParcelStore(db)
	parcel := getTestParcel()

	// add
	number, err := store.Add(parcel)
	require.NoError(t, err)
	require.NotEmpty(t, number)

	parcel.Number = number

	// set address
	newAddress := "new test address"
	err = store.SetAddress(parcel.Number, newAddress)
	require.NoError(t, err)

	// check
	res, err := store.Get(parcel.Number)
	require.NoError(t, err)

	require.Equal(t, newAddress, res.Address)

	// дополнительная проверка, что остальные поля не повредились при обновлении.
	require.Equal(t, parcel.Number, res.Number)
	require.Equal(t, parcel.Status, res.Status)
	require.Equal(t, parcel.Client, res.Client)
	require.Equal(t, parcel.CreatedAt, res.CreatedAt)
}

// TestSetStatus проверяет обновление статуса
func TestSetStatus(t *testing.T) {
	// arrange
	db := setupDB(t)
	defer db.Close()

	store := NewParcelStore(db)
	parcel := getTestParcel()

	// add
	number, err := store.Add(parcel)
	require.NoError(t, err)
	require.NotEmpty(t, number)

	parcel.Number = number

	// set status
	newStatus := Sent
	err = store.SetStatus(parcel.Number, newStatus)
	require.NoError(t, err)

	// check
	res, err := store.Get(parcel.Number)
	require.NoError(t, err)
	require.Equal(t, newStatus, res.Status)

	// дополнительная проверка, что остальные поля не повредились при аплейте.
	require.Equal(t, parcel.Number, res.Number)
	require.Equal(t, parcel.Client, res.Client)
	require.Equal(t, parcel.Address, res.Address)
	require.Equal(t, parcel.CreatedAt, res.CreatedAt)
}

// TestGetByClient проверяет получение посылок по идентификатору клиента
func TestGetByClient(t *testing.T) {
	// arrange
	db := setupDB(t)
	defer db.Close()

	store := NewParcelStore(db)

	parcels := []Parcel{
		getTestParcel(),
		getTestParcel(),
		getTestParcel(),
	}
	parcelMap := map[int]Parcel{}

	// задаём всем посылкам один и тот же идентификатор клиента
	client := randRange.Intn(10_000_000) + 1
	parcels[0].Client = client
	parcels[1].Client = client
	parcels[2].Client = client

	// add
	for i := 0; i < len(parcels); i++ {
		number, err := store.Add(parcels[i])
		require.NoError(t, err)
		require.NotEmpty(t, number)

		parcels[i].Number = number
		parcelMap[number] = parcels[i]
	}

	// get by client
	storedParcels, err := store.GetByClient(client)
	require.NoError(t, err)
	require.Len(t, storedParcels, len(parcels))

	// check
	for _, parcel := range storedParcels {
		expectedParcel, ok := parcelMap[parcel.Number]
		require.True(t, ok)
		require.Equal(t, expectedParcel, parcel)

	}
}

// Добавила негативные тесты и тест на валидацию входных данных.
func TestDeleteInvalidStatus(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	store := NewParcelStore(db)

	parcel := getTestParcel()
	number, _ := store.Add(parcel)

	err := store.SetStatus(number, Sent)
	require.NoError(t, err)

	err = store.Delete(number)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot delete")

	res, err := store.Get(number)
	require.NoError(t, err)
	require.Equal(t, number, res.Number)
}

func TestSetAddressInvalidStatus(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	store := NewParcelStore(db)

	parcel := getTestParcel()
	number, err := store.Add(parcel)
	require.NoError(t, err)

	err = store.SetStatus(number, Delivered)
	require.NoError(t, err)

	err = store.SetAddress(number, "Hogwarts School of Witchcraft and Wizardry")

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot change address")

	res, err := store.Get(number)
	require.NoError(t, err)
	require.Equal(t, parcel.Address, res.Address)
}

func TestGetNonExistent(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	store := NewParcelStore(db)

	testNum := 999999
	_, err := store.Get(testNum)

	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestAddValidationError(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	store := NewParcelStore(db)

	type testCase struct {
		name        string
		client      int
		address     string
		status      ParcelStatus
		expectedErr string
	}

	tests := []testCase{
		{
			name:        "empty address",
			client:      1,
			address:     "",
			status:      Registered,
			expectedErr: "address can not be empty",
		},
		{
			name:        "address too long",
			client:      1,
			address:     strings.Repeat("a", 666),
			status:      Registered,
			expectedErr: "address exceeds max len",
		},
		{
			name:        "invalid client ID",
			client:      0,
			address:     "Bag End",
			status:      Registered,
			expectedErr: "client ID must be > 0",
		},
		{
			name:        "invalid status",
			client:      1,
			address:     "Gingerbread House",
			status:      "wrong_status",
			expectedErr: "invalid parcel status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Parcel{
				Client:  tc.client,
				Address: tc.address,
				Status:  tc.status,
			}
			_, err := store.Add(p)

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
