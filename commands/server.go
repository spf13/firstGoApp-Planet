// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/GeertJohan/go.rice"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/mgo.v2/bson"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server for feeds",
	Long:  `Dagobah will serve all feeds listed in the config file.`,
	Run:   serverRun,
}

func init() {
	serverCmd.Flags().Int("port", 1138, "Port to run Dagobah server on")
	viper.BindPFlag("port", serverCmd.Flags().Lookup("port"))
}

func serverRun(cmd *cobra.Command, args []string) {
	r := gin.Default()
	templates := loadTemplates("full.html")
	r.HTMLTemplates = templates

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.GET("/", homeRoute)
	r.GET("/static/*filepath", staticServe)
	r.GET("/channel/*key", channelRoute)
	r.GET("/post/*key", postRoute)

	port := viper.GetString("port")
	fmt.Println("Running on port:", port)
	r.Run(":" + port)
}

func four04(c *gin.Context, message string) {
	c.HTML(404, "full.html", gin.H{"message": message, "title": message})
}

func homeRoute(c *gin.Context) {
	var posts []Itm
	results := Items().Find(bson.M{}).Sort("-date").Limit(20)
	results.All(&posts)

	obj := gin.H{"title": "Go Rules", "posts": posts, "channels": AllChannels()}
	c.HTML(200, "full.html", obj)
}

func staticServe(c *gin.Context) {
	static, err := rice.FindBox("static")
	if err != nil {
		log.Fatal(err)
	}

	original := c.Req.URL.Path
	c.Req.URL.Path = c.Params.ByName("filepath")
	fmt.Println(c.Params.ByName("filepath"))
	http.FileServer(static.HTTPBox()).ServeHTTP(c.Writer, c.Req)
	c.Req.URL.Path = original
}

func channelRoute(c *gin.Context) {
	key := c.Params.ByName("key")
	if len(key) < 2 {
		four04(c, "Channel Not Found")
		return
	}

	key = key[1:]

	fmt.Println(key)

	var posts []Itm
	results := Items().Find(bson.M{"channelkey": key}).Sort("-date").Limit(20)
	results.All(&posts)

	if len(posts) == 0 {
		four04(c, "No Articles")
		return
	}

	var currentChannel Chnl
	err := Channels().Find(bson.M{"key": key}).One(&currentChannel)
	if err != nil {
		if string(err.Error()) == "not found" {
			four04(c, "Channel not found")
			return
		} else {
			fmt.Println(err)
		}
	}

	obj := gin.H{"title": currentChannel.Title, "header": currentChannel.Title, "posts": posts, "channels": AllChannels()}

	c.HTML(200, "full.html", obj)
}

func postRoute(c *gin.Context) {
	key := c.Params.ByName("key")

	if len(key) < 2 {
		four04(c, "Invalid Post")
		return
	}

	key = key[1:]

	var ps []Itm
	r := Items().Find(bson.M{"key": key}).Sort("-date").Limit(1)
	r.All(&ps)

	if len(ps) == 0 {
		four04(c, "Post not found")
		return
	}

	var posts []Itm
	results := Items().Find(bson.M{"date": bson.M{"$lte": ps[0].Date}}).Sort("-date").Limit(20)
	results.All(&posts)

	obj := gin.H{"title": ps[0].Title, "posts": posts, "channels": AllChannels()}

	c.HTML(200, "full.html", obj)
}

func loadTemplates(list ...string) *template.Template {
	templateBox, err := rice.FindBox("templates")
	if err != nil {
		log.Fatal(err)
	}

	templates := template.New("")

	for _, x := range list {
		templateString, err := templateBox.String(x)
		if err != nil {
			log.Fatal(err)
		}

		// get file contents as string
		_, err = templates.New(x).Parse(templateString)
		if err != nil {
			log.Fatal(err)
		}
	}

	funcMap := template.FuncMap{
		"html":  ProperHtml,
		"title": func(a string) string { return strings.Title(a) },
	}

	templates.Funcs(funcMap)

	return templates
}

func ProperHtml(text string) template.HTML {
	if strings.Contains(text, "content:encoded>") || strings.Contains(text, "content/:encoded>") {
		text = html.UnescapeString(text)
	}
	return template.HTML(html.UnescapeString(template.HTMLEscapeString(text)))
}
