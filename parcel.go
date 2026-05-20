package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

	p.Address = strings.TrimSpace(p.Address)
	trimmedStatus := strings.TrimSpace(string(p.Status))
	p.Status = ParcelStatus(trimmedStatus)

	if p.Client <= 0 {
		return 0, errors.New("validation error: client ID must be > 0")
	}
	if p.Address == "" {
		return 0, errors.New("validation error: address can not be empty")
	}
	if len(p.Address) > 512 {
		return 0, errors.New("validation error: address exceeds max len = 512 characters")
	}

	if !IsValidStatus(p.Status) {
		return 0, fmt.Errorf("validation error: invalid parcel status %q", p.Status)
	}

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

	err := s.db.QueryRow(query, sql.Named("number", number)).Scan(&p.Number,
		&p.Client,
		&statusStr,
		&p.Address,
		&p.CreatedAt)

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

	if client <= 0 {
		return nil, errors.New("validation error: client ID must be > 0")
	}

	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s FROM %s WHERE %s = :client",
		colNumber, colClient, colStatus, colAddress, colCreatedAt, tableParcel, colClient,
	)

	rows, err := s.db.Query(query, sql.Named("client", client))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	defer rows.Close()

	res := []Parcel{}

	for rows.Next() {
		p := Parcel{}
		var statusStr string

		err := rows.Scan(
			&p.Number,
			&p.Client,
			&statusStr,
			&p.Address,
			&p.CreatedAt,
		)
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

	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}
	pStatus := ParcelStatus(strings.TrimSpace(string(status)))

	if !IsValidStatus(ParcelStatus(pStatus)) {
		return fmt.Errorf("validation error: invalid parcel status %q", status)
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s = :status WHERE %s = :number",
		tableParcel, colStatus, colNumber,
	)

	res, err := s.db.Exec(query, sql.Named("status", string(pStatus)), sql.Named("number", number))
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {

		return fmt.Errorf("parcel %d not found: %w", number, sql.ErrNoRows)
	}

	return nil
}

func (s *ParcelStore) SetAddress(number int, address string) error {

	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}

	address = strings.TrimSpace(address)
	if address == "" {
		return errors.New("validation error: address cannot be empty")
	}
	if len(address) > 512 {
		return errors.New("validation error: address exceeds 512 characters")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.checkStatus(tx, number, "change address"); err != nil {
		return err
	}

	queryUpdate := fmt.Sprintf(
		"UPDATE %s SET %s = :address WHERE %s = :number",
		tableParcel, colAddress, colNumber,
	)
	_, err = tx.Exec(queryUpdate, sql.Named("address", address), sql.Named("number", number))
	if err != nil {
		return fmt.Errorf("failed to update address: %w", err)
	}

	return tx.Commit()

}

func (s *ParcelStore) Delete(number int) error {
	if number <= 0 {
		return errors.New("validation error: parcel number must be > 0")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.checkStatus(tx, number, "delete"); err != nil {
		return err
	}

	queryDelete := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = :number ",
		tableParcel, colNumber,
	)
	_, err = tx.Exec(queryDelete, sql.Named("number", number))

	if err != nil {
		return fmt.Errorf("failed to delete parcel: %w", err)
	}
	return tx.Commit()
}

func (s *ParcelStore) checkStatus(tx *sql.Tx, number int, action string) error {
	var currentStatus string

	queryCheck := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = :number",
		colStatus, tableParcel, colNumber,
	)

	err := tx.QueryRow(queryCheck, sql.Named("number", number)).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parcel %d not found", number)
		}
		return fmt.Errorf("failed to get parcel status: %w", err)
	}

	if currentStatus != string(Registered) {
		return fmt.Errorf("cannot %s: parcel %d has status %q (expected %q)",
			action, number, currentStatus, Registered)
	}

	return nil
}
