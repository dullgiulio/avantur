package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	stmtAdd      *sql.Stmt
	stmtDelStage *sql.Stmt
	stmtDelClean *sql.Stmt
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
	go m.refreshStmts(30 * time.Minute) // TODO: configurable
	return m, nil
}

func (m *Mysql) refreshStmts(d time.Duration) {
	c := time.Tick(d)
	for range c {
		if err := m.initStmts(); err != nil {
			log.Printf("[mysql] error while creating prepared statements: %s", err)
		}
	}
}

func (m *Mysql) initStmts() error {
	var err error
	m.mux.Lock()
	defer m.mux.Unlock()
	if _, err = m.db.Exec(createTable); err != nil {
		return fmt.Errorf("cannot create storage table: %s", err)
	}
	if m.stmtAdd, err = m.db.Prepare(fmt.Sprintf(queryAdd, m.tableName)); err != nil {
		return fmt.Errorf("cannot prepare add statement: %s", err)
	}
	if m.stmtDelStage, err = m.db.Prepare(fmt.Sprintf(queryDeleteStage, m.tableName)); err != nil {
		return fmt.Errorf("cannot prepare delete by stage statement: %s", err)
	}
	if m.stmtDelClean, err = m.db.Prepare(fmt.Sprintf(queryDeleteClean, m.tableName)); err != nil {
		return fmt.Errorf("cannot prepare delete by date statement: %s", err)
	}
	return nil
}

func (m *Mysql) Add(br *BuildResult) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, err := m.stmtAdd.Exec(br.Start, br.End, br.Act, br.Ticket, br.Retval, br.SHA1,
		br.Stage, br.Cmd, br.Branch, br.Stdout, br.Stderr)
	return err
}

func (m *Mysql) Get(stage string) ([]*BuildResult, error) {
	// TODO
	return nil, errors.New("not implemented")
}

func (m *Mysql) Delete(stage string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, err := m.stmtDelStage.Exec(stage)
	return err
}

func (m *Mysql) Clean(until time.Time) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, err := m.stmtDelClean.Exec(until)
	return err
}
