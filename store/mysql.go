package store

import (
	"database/sql"
	"errors"
	"fmt"

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
  branch text NOT NULL,
  stdout text NOT NULL,
  stderr text NOT NULL,
  PRIMARY KEY (id)
);
`

const (
	queryAdd         = `INSERT INTO %s (start,end,act,ticket,exitcode,sha1,stage,branch,stdout,stderr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	queryDeleteStage = `DELETE FROM %s WHERE stage = ?`
	// queryGet = TODO
)

type Mysql struct {
	db           *sql.DB
	tableName    string
	stmtAdd      *sql.Stmt
	stmtDelStage *sql.Stmt
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
	if _, err = m.db.Exec(createTable); err != nil {
		return fmt.Errorf("cannot create storage table: %s", err)
	}
	if m.stmtAdd, err = m.db.Prepare(fmt.Sprintf(queryAdd, m.tableName)); err != nil {
		return fmt.Errorf("cannot prepare add statement: %s", err)
	}
	if m.stmtDelStage, err = m.db.Prepare(fmt.Sprintf(queryDeleteStage, m.tableName)); err != nil {
		return fmt.Errorf("cannot prepare delete by stage statement: %s", err)
	}
	return nil
}

func (m *Mysql) Add(br *BuildResult) error {
	_, err := m.stmtAdd.Exec(br.Start, br.End, br.Act, br.Ticket, br.Retval, br.SHA1, br.Stage, br.Branch, br.Stdout, br.Stderr)
	return err
}

func (m *Mysql) Get(stage string) ([]*BuildResult, error) {
	// TODO
	return nil, errors.New("not implemented")
}

func (m *Mysql) Delete(stage string) error {
	_, err := m.stmtDelStage.Exec(stage)
	return err
}
