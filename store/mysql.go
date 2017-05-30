// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const createTable = `
CREATE TABLE IF NOT EXISTS build_results (
  id int(11) NOT NULL AUTO_INCREMENT,
  start datetime NOT NULL,
  end datetime NOT NULL,
  act int(11) NOT NULL,
  ticket int(11) NOT NULL,
  exitcode int(11) NOT NULL,
  sha1 char(40) NOT NULL,
  stage varchar(250) NOT NULL,
  cmd text NOT NULL,
  branch text NOT NULL,
  stdout text NOT NULL,
  stderr text NOT NULL,
  PRIMARY KEY (id)
);
`

const (
	queryAdd         = `INSERT INTO %s (start,end,act,ticket,exitcode,sha1,stage,cmd,branch,stdout,stderr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	queryDeleteStage = `DELETE FROM %s WHERE stage = ?`
	queryDeleteClean = `DELETE FROM %s WHERE end < ?`
	// queryGet = TODO
)

type Mysql struct {
	mux          sync.Mutex
	db           *sql.DB
	tableName    string
	stmtAdd      string
	stmtDelStage string
	stmtDelClean string
}

func NewMysql(dsn, tableName string) (*Mysql, error) {
	var err error
	m := &Mysql{tableName: tableName}
	if m.db, err = sql.Open("mysql", dsn); err != nil {
		return nil, err
	}
	if err = m.initStmts(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Mysql) initStmts() error {
	var err error
	m.mux.Lock()
	defer m.mux.Unlock()
	if _, err = m.db.Exec(createTable); err != nil {
		return fmt.Errorf("cannot create storage table: %s", err)
	}
	m.stmtAdd = fmt.Sprintf(queryAdd, m.tableName)
	m.stmtDelStage = fmt.Sprintf(queryDeleteStage, m.tableName)
	m.stmtDelClean = fmt.Sprintf(queryDeleteClean, m.tableName)
	return nil
}

func (m *Mysql) Add(br *BuildResult) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	r, err := m.db.Query(m.stmtAdd, br.Start, br.End, br.Act, br.Ticket, br.Retval, br.SHA1,
		br.Stage, br.Cmd, br.Branch, br.Stdout, br.Stderr)
	r.Close()
	return err
}

func (m *Mysql) Get(stage string) ([]*BuildResult, error) {
	// TODO
	return nil, errors.New("not implemented")
}

func (m *Mysql) Delete(stage string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	r, err := m.db.Query(m.stmtDelStage, stage)
	r.Close()
	return err
}

func (m *Mysql) Clean(until time.Time) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	r, err := m.db.Query(m.stmtDelClean, until)
	r.Close()
	return err
}
