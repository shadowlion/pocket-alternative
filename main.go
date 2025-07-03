package main

import (
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
)

func main() {

	app := pocketbase.New()

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		registry := template.NewRegistry()

		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		se.Router.GET("/{$}", func(e *core.RequestEvent) error {
			return PageHandlerHome(e, registry)
		})

		se.Router.GET("/login", func(e *core.RequestEvent) error {
			return PageHandlerLogin(e, registry)
		})

		se.Router.POST("/submit", FormHandlerLogin)

		apiRoutes := se.Router.Group("/api")
		apiRoutes.GET("/v1/health", ApiHandlerHealthCheck)
		apiRoutes.POST("/v1/processLink", func(e *core.RequestEvent) error {
			return ApiHandlerProcessLink(app, e)
		})

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func PageHandlerHome(e *core.RequestEvent, registry *template.Registry) error {
	html, err := registry.LoadFiles(
		"views/layout.html",
		"views/index.html",
	).Render(map[string]any{})

	if err != nil {
		// or redirect to a dedicated 404 HTML page
		return e.NotFoundError("", err)
	}

	return e.HTML(http.StatusOK, html)
}

func PageHandlerLogin(e *core.RequestEvent, registry *template.Registry) error {
	html, err := registry.LoadFiles(
		"views/layout.html",
		"views/login.html",
	).Render(map[string]any{})

	if err != nil {
		// or redirect to a dedicated 404 HTML page
		return e.NotFoundError("", err)
	}

	return e.HTML(http.StatusOK, html)
}

func FormHandlerLogin(e *core.RequestEvent) error {
	email := e.Request.FormValue("email")
	password := e.Request.FormValue("password")

	log.Printf("Received form submission - Email: %s, Password: %s", email, password)

	return e.Redirect(http.StatusPermanentRedirect, "")
}

func PageHandlerUpload(e *core.RequestEvent, registry *template.Registry) error {
	html, err := registry.LoadFiles(
		"views/layout.html",
		"views/upload.html",
	).Render(map[string]any{})

	if err != nil {
		// or redirect to a dedicated 404 HTML page
		return e.NotFoundError("", err)
	}

	return e.HTML(http.StatusOK, html)
}

func ApiHandlerHealthCheck(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"message": "Ok"})
}

func ApiHandlerProcessLink(app *pocketbase.PocketBase, e *core.RequestEvent) error {
	data := struct {
		URL string `json:"url"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Failed to read request body", err)
	}

	info, err := e.RequestInfo()
	if err != nil {
		return e.BadRequestError("Failed to read request body", err)
	}

	url, ok := info.Body["url"].(string)
	if !ok {
		return e.BadRequestError("Failed to read request body", err)
	}

	rec, err := createNewRecord(app, "articles")
	if err != nil {
		return e.BadRequestError("Failed to create database record", err)
	}

	article := NewArticle(url)
	rec.Set("title", article.title)
	rec.Set("content", article.content)
	if err := app.Save(rec); err != nil {
		return e.BadRequestError("Failed to save to database", err)
	}

	return e.JSON(http.StatusOK, map[string]any{"message": "Ok", "id": rec.Id})
}

func createNewRecord(app *pocketbase.PocketBase, colId string) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId(colId)
	if err != nil {
		return nil, err
	}

	record := core.NewRecord(collection)
	return record, nil
}

type Article struct {
	title   string
	content string
}

func NewArticle(url string) Article {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("style, noscript, script").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	title := doc.Find("article>h1").First().Text()
	rawContent := doc.Find("article").First().Text()
	re := regexp.MustCompile(`\s+`)
	cleanText := re.ReplaceAllString(rawContent, " ")
	trimmedText := strings.TrimSpace(cleanText)
	maxLen := 5000
	if len(trimmedText) > maxLen {
		trimmedText = trimmedText[:maxLen]
	}

	return Article{
		title:   title,
		content: trimmedText,
	}
}
