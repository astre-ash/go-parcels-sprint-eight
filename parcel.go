package main

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	dbName      = "tracker.db"
	tableParcel = "parcel"

	colNumber    = "number"
	colClient    = "client"
	colStatus    = "status"
	colAddress   = "address"
	colCreatedAt = "created_at"
)

type ParcelStore struct {
	db *sql.DB
}

func NewParcelStore(db *sql.DB) *ParcelStore {
	return &ParcelStore{db: db}
}

func (s *ParcelStore) Add(p Parcel) (int, error) {
	query := fmt.Sprintf(
		"INSERT INTO %s (%s, %s, %s, %s) VALUES (:client, :status, :address, :created_at)",
		tableParcel, colClient, colStatus, colAddress, colCreatedAt,
	)

	res, err := s.db.Exec(query,
		sql.Named("client", p.Client),
		sql.Named("status", string(p.Status)),
		sql.Named("address", p.Address),
		sql.Named("created_at", p.CreatedAt))

	if err != nil {
		return 0, fmt.Errorf("failed executing request: %w", err)
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed getting ID: %w", err)
	}

	return int(lastId), nil
}

func (s *ParcelStore) Get(number int) (Parcel, error) {
	p := Parcel{}
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s FROM %s WHERE %s = :number",
		colNumber, colClient, colStatus, colAddress, colCreatedAt, tableParcel, colNumber)

	var statusStr string
	err := s.db.QueryRow(query, sql.Named("number", number)).Scan(
		&p.Number,
		&p.Client,
		&statusStr,
		&p.Address,
		&p.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Parcel{}, fmt.Errorf("parcel with number %d not found: %w", number, err)
		}
		return p, fmt.Errorf("failed to get parcel: %w", err)
	}
	p.Status = ParcelStatus(statusStr)

	return p, nil
}

func (s *ParcelStore) GetByClient(client int) ([]Parcel, error) {
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s FROM %s WHERE %s = :client",
		colNumber, colClient, colStatus, colAddress, colCreatedAt, tableParcel, colClient,
	)

	rows, err := s.db.Query(query, sql.Named("client", client))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var res []Parcel
	for rows.Next() {
		p := Parcel{}
		var statusStr string
		err := rows.Scan(&p.Number, &p.Client, &statusStr, &p.Address, &p.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		p.Status = ParcelStatus(statusStr)
		res = append(res, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return res, nil
}

func (s *ParcelStore) SetStatus(number int, status ParcelStatus) error {
	query := fmt.Sprintf("UPDATE %s SET %s = :status WHERE %s = :number", tableParcel, colStatus, colNumber)

	res, err := s.db.Exec(query, sql.Named("status", string(status)), sql.Named("number", number))
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("parcel %d not found", number)
	}

	return nil
}

func (s *ParcelStore) SetAddress(number int, address string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	queryCheck := fmt.Sprintf("SELECT %s FROM %s WHERE %s = :number", colStatus, tableParcel, colNumber)

	err = tx.QueryRow(queryCheck, sql.Named("number", number)).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parcel %d not found", number)
		}
		return fmt.Errorf("failed to get parcel status: %w", err)
	}

	if currentStatus != string(Registered) {
		return fmt.Errorf("cannot update address: parcel %d has status %q (expected %q)",
			number, currentStatus, Registered)
	}

	queryUpdate := fmt.Sprintf("UPDATE %s SET %s = :address WHERE %s = :number", tableParcel, colAddress, colNumber)

	_, err = tx.Exec(queryUpdate, sql.Named("address", address), sql.Named("number", number))
	if err != nil {
		return fmt.Errorf("failed to update address: %w", err)
	}

	return tx.Commit()
}

func (s *ParcelStore) Delete(number int) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = :number AND %s = :status",
		tableParcel, colNumber, colStatus,
	)

	res, err := s.db.Exec(query,
		sql.Named("number", number),
		sql.Named("status", string(Registered)),
	)
	if err != nil {
		return fmt.Errorf("failed to delete parcel: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("parcel %d not found or cannot be deleted (wrong status)", number)
	}

	return nil
}
