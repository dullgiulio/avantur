package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const createTable = `
CREATE TABLE IF NOT EXISTS build_results (
  id int(11) NOT NULL AUTO_INCREMENT,
  start date NOT NULL,
  end date NOT NULL,
  env varchar(200) NOT NULL,
  ticket int(11) NOT NULL,
  exitcode int(11) NOT NULL,
  stdout text NOT NULL,
  stderr text NOT NULL,
  PRIMARY KEY (id)
);
`

const (
	queryAdd          = `INSERT INTO %s (start,end,env,ticket,exitcode,stdout,stderr) VALUES (?, ?, ?, ?, ?, ?, ?)`
	queryDeleteTicket = `DELETE FROM %s WHERE ticket = ?`
	queryDeleteEnv    = `DELETE FROM %s WHERE env = ?`
	// queryGet = TODO
)

type Mysql struct {
	db            *sql.DB
	tableName     string
	stmtAdd       *sql.Stmt
	stmtDelTicket *sql.Stmt
	stmtDelEnv    *sql.Stmt
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
	if m.stmtAdd, err = m.db.Prepare(fmt.Sprintf(queryAdd, m.tableName)); err != nil {
		return err
	}
	if m.stmtDelTicket, err = m.db.Prepare(fmt.Sprintf(queryDeleteTicket, m.tableName)); err != nil {
		return err
	}
	if m.stmtDelEnv, err = m.db.Prepare(fmt.Sprintf(queryDeleteEnv, m.tableName)); err != nil {
		return err
	}
	return nil
}

func (m *Mysql) Add(env string, ticket int64, br *BuildResult) error {
	_, err := m.stmtAdd.Exec(&time.Time{}, &time.Time{}, env, ticket, br.Retval, br.Stdout, br.Stderr)
	return err
}

func (m *Mysql) Get(env string, ticket int64) ([]*BuildResult, error) {
	// TODO
	return nil, errors.New("not implemented")
}

func (m *Mysql) DeleteTicket(env string, ticket int64) error {
	_, err := m.stmtDelTicket.Exec(ticket)
	return err
}

func (m *Mysql) DeleteEnv(env string) error {
	_, err := m.stmtDelEnv.Exec(env)
	return err
}
