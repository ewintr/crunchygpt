package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	"golang.org/x/exp/slog"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	openaiApiKey, ok := os.LookupEnv("OPENAI_API_KEY")
	if !ok {
		logger.Error("OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	// setup weaviate client
	cfg := weaviate.Config{
		Host:   "192.168.1.209:8080", // Replace with your endpoint
		Scheme: "http",
		Headers: map[string]string{
			"X-OpenAI-Api-Key": openaiApiKey, // Replace with your inference API key
		},
	}
	client, err := weaviate.NewClient(cfg)
	if err != nil {
		logger.Error("Could not create client", err)
		os.Exit(1)
	}

	// remove the class if it already exists
	className := "Question"

	// delete the class
	if err := client.Schema().ClassDeleter().WithClassName(className).Do(context.Background()); err != nil {
		// Weaviate will return a 400 if the class does not exist, so this is allowed, only return an error if it's not a 400
		if status, ok := err.(*fault.WeaviateClientError); ok && status.StatusCode != http.StatusBadRequest {
			panic(err)
		}
	}

	classObj := &models.Class{
		Class:      "Question",
		Vectorizer: "text2vec-openai", // Or "text2vec-cohere" or "text2vec-huggingface"
	}

	// add the schema
	if err := client.Schema().ClassCreator().WithClass(classObj).Do(context.Background()); err != nil {
		logger.Error("Could not create class", err)
		os.Exit(1)
	}

	// Retrieve the data
	data, err := http.DefaultClient.Get("https://raw.githubusercontent.com/weaviate-tutorials/quickstart/main/data/jeopardy_tiny.json")
	if err != nil {
		panic(err)
	}
	defer data.Body.Close()

	// Decode the data
	var items []map[string]string
	if err := json.NewDecoder(data.Body).Decode(&items); err != nil {
		panic(err)
	}

	// convert items into a slice of models.Object
	objects := make([]*models.Object, len(items))
	for i := range items {
		objects[i] = &models.Object{
			Class: "Question",
			Properties: map[string]any{
				"category": items[i]["Category"],
				"question": items[i]["Question"],
				"answer":   items[i]["Answer"],
			},
		}
	}

	// batch write items
	batchRes, err := client.Batch().ObjectsBatcher().WithObjects(objects...).Do(context.Background())
	if err != nil {
		logger.Error("Could not batch write objects", err)
		os.Exit(1)
	}
	for _, res := range batchRes {
		if res.Result.Errors != nil {
			logger.Error("Could not batch write objects", res.Result.Errors)
			os.Exit(1)
		}
	}

	// Retrieve the data
	fields := []graphql.Field{
		{Name: "question"},
		{Name: "answer"},
		{Name: "category"},
	}

	nearText := client.GraphQL().
		NearTextArgBuilder().
		WithConcepts([]string{"what about forecasting?"})

	result, err := client.GraphQL().Get().
		WithClassName("Question").
		WithFields(fields...).
		WithNearText(nearText).
		WithLimit(2).
		Do(context.Background())
	if err != nil {
		logger.Error("Could not get objects", err)
		os.Exit(1)
	}

	jsRes, err := json.Marshal(result.Data)
	if err != nil {
		logger.Error("Could not marshal json", err)
		os.Exit(1)
	}

	type Question struct {
		Answer   string `json:"answer"`
		Category string `json:"category"`
		Question string `json:"question"`
	}

	type JsonRequest struct {
		Get struct {
			Question []Question `json:"Question"`
		} `json:"Get"`
	}

	var res JsonRequest

	if err := json.Unmarshal(jsRes, &res); err != nil {
		logger.Error("Could not unmarshal json", err)
		os.Exit(1)
	}
	for _, res := range res.Get.Question {

		fmt.Printf("%s: %s (%s)\n\n", res.Question, res.Answer, res.Category)
	}
}
