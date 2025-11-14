package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	utils "github.com/sahilchouksey/go-init-setup/utils"
)

func GetAllTodos(c fiber.Ctx, store database.Storage) error {
	// Get the database connection from the context
	todos, err := store.GetTodos()
	if err != nil {
		return err
	}

	utils.WriteJSON(c, 200, todos, nil, nil)
	return nil
}

func AddTodoHandler(c fiber.Ctx, store database.Storage) error {
	var todo model.Todo
	var err error

	todo.Name = c.Query("name")
	todo.Description = c.Query("description")
	todo.Due, err = time.Parse(time.RFC3339, c.Query("due"))

	if err != nil {
		return err
	}

	if todo.Name == "" || todo.Description == "" {
		err := errors.New("missing some fields")
		return err
	}

	if err := store.AddTodo(todo); err != nil {
		return err
	}

	utils.WriteJSON(c, 200, todo, nil, nil)
	return nil
}

func UpdateTodoHandler(c fiber.Ctx, store database.Storage) error {
	var todo model.Todo
	var err error

	if todo.ID, err = strconv.ParseInt(c.Query("id"), 10, 64); todo.ID <= 0 || err != nil {
		err := errors.New(fmt.Sprintf("unable to find todo associated with %d", todo.ID))
		return err
	}
	todo.Name = c.Query("name")
	todo.Description = c.Query("description")
	if c.Query("due") != "" {
		if todo.Due, err = time.Parse(time.RFC3339, c.Query("due")); err != nil {
			return err
		}
	}
	todo.Status = c.Query("status")

	if err := store.UpdateTodo(todo); err != nil {
		return err
	}

	utils.WriteJSON(c, 201, todo, nil, nil)
	return nil
}

func DeleteTodoHandler(c fiber.Ctx, store database.Storage) error {
	var todoId int64
	var err error
	if todoId, err = strconv.ParseInt(c.Query("id"), 10, 64); todoId <= 0 || err != nil {
		err := errors.New(fmt.Sprintf("unable to find todo associated with %d", todoId))
		return err
	}

	if err := store.DeleteTodo(todoId); err != nil {
		return nil
	}

	utils.WriteJSON(c, 201, fmt.Sprintf("deleted todo : #%d", todoId), nil, nil)

	return nil
}
