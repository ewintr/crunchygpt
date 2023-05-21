package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"golang.org/x/exp/slog"
	"golang.org/x/net/html"
)

const (
	className = "Recipe"
)

type Recipe struct {
	Title string
	Body  string
}

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

	//recipes, err := ProcessHTMLFiles("./content")
	//if err != nil {
	//	logger.Error("Could not process HTML files", err)
	//	os.Exit(1)
	//}
	//
	//// delete the class if it already exists
	//if err := client.Schema().ClassDeleter().WithClassName(className).Do(context.Background()); err != nil {
	//	// Weaviate will return a 400 if the class does not exist, so this is allowed, only return an error if it's not a 400
	//	if status, ok := err.(*fault.WeaviateClientError); ok && status.StatusCode != http.StatusBadRequest {
	//		panic(err)
	//	}
	//}
	//
	//classObj := &models.Class{
	//	Class:      className,
	//	Vectorizer: "text2vec-openai", // Or "text2vec-cohere" or "text2vec-huggingface"
	//}
	//
	//// add the schema
	//if err := client.Schema().ClassCreator().WithClass(classObj).Do(context.Background()); err != nil {
	//	logger.Error("Could not create class", err)
	//	os.Exit(1)
	//}
	//
	//// convert items into a slice of models.Object
	//objects := make([]*models.Object, len(recipes))
	//for i, recipe := range recipes {
	//	objects[i] = &models.Object{
	//		Class: className,
	//		Properties: map[string]any{
	//			"title": recipe.Title,
	//			"body":  recipe.Body,
	//		},
	//	}
	//}
	//
	//// batch write items
	//batchRes, err := client.Batch().ObjectsBatcher().WithObjects(objects...).Do(context.Background())
	//if err != nil {
	//	logger.Error("Could not batch write objects", err)
	//	os.Exit(1)
	//}
	//for _, res := range batchRes {
	//	if res.Result.Errors != nil {
	//		logger.Error("Could not batch write objects", res.Result.Errors)
	//		os.Exit(1)
	//	}
	//}

	// Retrieve the data
	fields := []graphql.Field{
		{Name: "title"},
		{Name: "body"},
	}

	nearText := client.GraphQL().
		NearTextArgBuilder().
		//WithConcepts([]string{"suikervrij, zonder suiker, natuurlijke zoetstoffen, stevia, suikervervanger, gezonde recepten, low carb, keto, diabetische recepten, suikerbewust"})
		WithConcepts([]string{"Paleo", "Graanvrij", "Zuivelvrij", "Zonder geraffineerde suikers", "Biologisch", "Glutenvrij", "Koolhydraatarm", "Vlees en vis", "Groenten en fruit", "Noten en zaden", "Honing of ahornsiroop"})

	result, err := client.GraphQL().Get().
		WithClassName(className).
		WithFields(fields...).
		WithNearText(nearText).
		WithLimit(25).
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
	//fmt.Printf("body: %s\n\n", string(jsRes))
	//return

	type JsonRequest struct {
		Get struct {
			Recipe []Recipe `json:"Recipe"`
		} `json:"Get"`
	}

	var res JsonRequest

	if err := json.Unmarshal(jsRes, &res); err != nil {
		logger.Error("Could not unmarshal json", err)
		os.Exit(1)
	}
	for i, res := range res.Get.Recipe {

		fmt.Printf("%d: %s \n\n", i, res.Title)
	}
}

func ProcessHTMLFiles(dir string) ([]Recipe, error) {
	recipes := []Recipe{}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			rec := ExtractTextFromHTML(string(data))
			recipes = append(recipes, rec)
			//fmt.Printf("Text content of %s:\n%s\n\n", path, text)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return recipes, nil
}

func ExtractTextFromHTML(content string) Recipe {
	var textBuilder strings.Builder
	var titleBuilder strings.Builder
	r := strings.NewReader(content)
	z := html.NewTokenizer(r)

	var inH1 bool

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken: // End of the document, break the loop
			return Recipe{
				Title: titleBuilder.String(),
				Body:  textBuilder.String(),
			}
		case html.StartTagToken:
			tn, _ := z.TagName()
			if string(tn) == "h1" {
				inH1 = true
			}
		case html.EndTagToken:
			tn, _ := z.TagName()
			if string(tn) == "h1" {
				inH1 = false
			}
		case html.TextToken:
			if inH1 {
				if txt := strings.TrimSpace(string(z.Text())); len(txt) > 0 {
					titleBuilder.WriteString(txt)
				}
			} else {
				if txt := strings.TrimSpace(string(z.Text())); len(txt) > 0 {
					textBuilder.WriteString(txt)
				}
			}
		}
	}
}
