package database

import (
	"database/sql"
	"fmt"

	"github.com/sahilchouksey/go-init-setup/model"
	queryHelper "github.com/sahilchouksey/go-init-setup/utils/query"
)

func (s *PostgreSQLStore) GetTodos() ([]model.Todo, error) {
	query := `
		SELECT id, name, description, status, due AS due_date FROM todo;
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	todos := []model.Todo{}
	for rows.Next() {
		todo, err := scanIntoTodo(rows)
		if err != nil {
			return nil, err
		}
		todos = append(todos, *todo)
	}

	return todos, nil
}

func (s *PostgreSQLStore) AddTodo(todo model.Todo) error {

	query := `INSERT INTO todo(name, description, due) VALUES($1, $2, $3);`

	sqlFormmatedDate := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", todo.Due.Year(), todo.Due.Month(), todo.Due.Day(), todo.Due.Hour(), todo.Due.Minute(), todo.Due.Second())

	_, err := s.db.Query(query, todo.Name, todo.Description, sqlFormmatedDate)

	if err != nil {
		return err
	}
	return nil

}

func (s *PostgreSQLStore) UpdateTodo(todo model.Todo) error {
	query, values := queryHelper.UpdateQueryBuilder("todo", "id", todo.ID, todo)

	_, err := s.db.Query(query, values...)

	if err != nil {
		return err
	}
	return nil
}

func (s *PostgreSQLStore) DeleteTodo(todoId int64) error {
	query := "DELETE FROM todo WHERE id=$1"

	if _, err := s.db.Query(query, todoId); err != nil {
		return err
	}

	return nil
}

func scanIntoTodo(rows *sql.Rows) (*model.Todo, error) {
	todo := new(model.Todo)
	err := rows.Scan(
		&todo.ID,
		&todo.Name,
		&todo.Description,
		&todo.Status,
		&todo.Due,
	)
	if err != nil {
		return nil, err
	}
	return todo, nil
}
