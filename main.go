package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

// Список поздравлений (индекс 0 соответствует ID 1 и т.д.)
var greetings = []string{
	"С 8 Марта! Пусть каждый день дарит улыбки, радость и вдохновение!",
	"Поздравляю с Международным женским днём! Желаю весеннего настроения, любви и счастья!",
	"С 8 Марта! Оставайтесь такой же прекрасной, нежной и удивительной!",
	"Пусть в этот день сбудутся самые заветные мечты. С праздником весны!",
	"С 8 Марта! Желаю море цветов, тепла, уюта и приятных сюрпризов!",
	"Поздравляю с днём очарования! Будьте счастливы, любимы и неповторимы!",
	"С Международным женским днём! Пусть весна расцветает в душе, а сердце согревает любовь.",
	"С 8 Марта! Желаю, чтобы каждый день был таким же ярким и прекрасным, как первые весенние цветы.",
	"Поздравляю с праздником! Пусть жизнь играет яркими красками, а рядом будут только верные и любящие люди.",
	"С 8 Марта! Желаю женского счастья, крепкого здоровья и исполнения желаний!",
}

// Список цветов для каждого ID (эмодзи)
var flowers = []string{
	"🌷🌹🌸",
	"🌼🌻🌺",
	"🌷🌷🌷",
	"🌸🌸🌸",
	"🌹🌹🌹",
	"🌺🌺🌺",
	"🌻🌻🌻",
	"🌼🌼🌼",
	"🌷🌹🌺",
	"🌸🌼🌻",
}

// Структура, представляющая ответ с поздравлением и цветами
type GreetingResponse struct {
	Text    string `json:"text"`
	Flowers string `json:"flowers"`
}

func main() {
	// 1. Определяем объектный тип Greeting
	greetingType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Greeting",
		Fields: graphql.Fields{
			"text": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
			},
			"flowers": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
			},
		},
	})

	// 2. Поле greeting в корневом запросе
	greetingField := &graphql.Field{
		Type: greetingType,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id, ok := p.Args["id"].(int)
			if !ok {
				return nil, fmt.Errorf("id должен быть целым числом")
			}
			if id < 1 || id > len(greetings) {
				return nil, fmt.Errorf("поздравление с ID %d не найдено", id)
			}
			// Индексация с 0
			return GreetingResponse{
				Text:    greetings[id-1],
				Flowers: flowers[id-1],
			}, nil
		},
	}

	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"greeting": greetingField,
		},
	})

	schemaConfig := graphql.SchemaConfig{Query: rootQuery}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		log.Fatalf("ошибка создания схемы GraphQL: %v", err)
	}

	// 3. Создаём HTTP-обработчик с включённым GraphiQL
	graphqlHandler := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})

	// 4. Определяем порт из окружения или используем 8080 по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{Addr: ":" + port, Handler: graphqlHandler}
	go func() {
		log.Printf("GraphQL сервер запущен на http://localhost:%s", port)
		log.Printf("GraphiQL интерфейс доступен по адресу http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ошибка запуска сервера: %v", err)
		}
	}()

	// 5. Ожидание сигнала завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 6. CLI-взаимодействие
	fmt.Println("Введите ID поздравления (от 1 до 10) для получения текста и цветов. Для выхода введите 'exit' или нажмите Ctrl+C.")
	for {
		fmt.Print("ID: ")
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			fmt.Println("Ошибка ввода, попробуйте снова")
			continue
		}
		if input == "exit" {
			fmt.Println("Завершение работы.")
			break
		}

		var id int
		_, err = fmt.Sscan(input, &id)
		if err != nil {
			fmt.Println("Пожалуйста, введите число от 1 до 10")
			continue
		}

		// Формируем GraphQL-запрос (теперь запрашиваем оба поля)
		query := fmt.Sprintf(`{"query": "query { greeting(id: %d) { text flowers } }"}`, id)
		body := bytes.NewBufferString(query)

		resp, err := http.Post(fmt.Sprintf("http://localhost:%s/", port), "application/json", body)
		if err != nil {
			log.Printf("Ошибка при отправке запроса: %v", err)
			continue
		}
		defer resp.Body.Close()

		var result struct {
			Data struct {
				Greeting struct {
					Text    string `json:"text"`
					Flowers string `json:"flowers"`
				} `json:"greeting"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("Ошибка декодирования ответа: %v", err)
			continue
		}

		if len(result.Errors) > 0 {
			fmt.Printf("Ошибка от сервера: %s\n", result.Errors[0].Message)
		} else {
			fmt.Printf("Поздравление: %s\n", result.Data.Greeting.Text)
			fmt.Printf("Цветы: %s\n\n", result.Data.Greeting.Flowers)
		}
	}

	// 7. Graceful shutdown
	fmt.Println("Останавливаем сервер...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при остановке сервера: %v", err)
	}
	fmt.Println("Сервер остановлен.")
}
