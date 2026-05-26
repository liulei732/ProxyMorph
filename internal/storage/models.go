package storage

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ConversionTask struct {
	ID                      int64
	UserID                  int64
	Name                    string
	InputType               string
	OutputType              string
	SourceURL               string
	Enabled                 bool
	RefreshIntervalSeconds  int
	MergeDefaultPinnedNodes bool
	PinnedNodeOrderMode     string
	LastSuccessAt           *time.Time
	LastErrorAt             *time.Time
	LastErrorMessage        string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type PinnedNode struct {
	ID             int64
	UserID         int64
	Name           string
	Protocol       string
	Server         string
	Port           int
	ParametersJSON string
	TagsJSON       string
	Enabled        bool
	DefaultInclude bool
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
