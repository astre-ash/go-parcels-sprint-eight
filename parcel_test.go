package main

import (
	"database/sql"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, parcel.Number, res.Number)
	assert.Equal(t, parcel.Client, res.Client)
	assert.Equal(t, parcel.Status, res.Status)
	assert.Equal(t, parcel.Address, res.Address)
	assert.Equal(t, parcel.CreatedAt, res.CreatedAt)

	// delete
	err = store.Delete(number)
	require.NoError(t, err)

	_, err = store.Get(number)
	assert.ErrorIs(t, err, sql.ErrNoRows)

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

	assert.Equal(t, newAddress, res.Address)

	// дополнительная проверка, что остальные поля не повредились при обновлении.
	assert.Equal(t, parcel.Number, res.Number)
	assert.Equal(t, parcel.Status, res.Status)
	assert.Equal(t, parcel.Client, res.Client)
	assert.Equal(t, parcel.CreatedAt, res.CreatedAt)
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
	assert.Equal(t, newStatus, res.Status)

	// дополнительная проверка, что остальные поля не повредились при аплейте.
	assert.Equal(t, parcel.Number, res.Number)
	assert.Equal(t, parcel.Client, res.Client)
	assert.Equal(t, parcel.Address, res.Address)
	assert.Equal(t, parcel.CreatedAt, res.CreatedAt)
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
		assert.True(t, ok)
		assert.Equal(t, expectedParcel, parcel)

	}
}
