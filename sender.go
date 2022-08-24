package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/time/rate"
)

var (
	database   string
	collection string
	wg         sync.WaitGroup
	cS         SpeedrunClient
	cD         DiscordClient
)

type DiscordClient struct {
	client  *http.Client
	limiter *rate.Limiter
}

type SpeedrunClient struct {
	client  *http.Client
	limiter *rate.Limiter
}

const (
	connectionStringEnv = "SRC_WEBHOOK_MONGODB_CONNECTION_STRING"
	databaseNameEnv     = "SRC_WEBHOOK_DATABASE"
	collectionNameEnv   = "SRC_WEBHOOK_INFO_COLLECTION"
)

type Webhook struct {
	WebhookUrl string `json:"WebhookUrl,omitempty"`
	Records    struct {
		Categories []string `json:"Categories,omitempty"`
		Users      []string `json:"Users,omitempty"`
		Events     string   `json:"Events,omitempty"`
	}
	Verification struct {
		Context string   `json:"Context,omitempty"`
		IDs     []string `json:"IDs,omitempty"`
		Events  []string `json:"Events,omitempty"`
	}
	PlayerIndex int `json:"PlayerIndex,omitempty"`
}

type Data struct {
	ID      string `json:"id,omitempty"`
	Order   int    `json:"order,omitempty"`
	New     bool   `json:"new,omitempty"`
	Weblink string `json:"weblink,omitempty"`
	Game    struct {
		Data struct {
			ID    string `json:"id,omitempty"`
			Names struct {
				International string `json:"international,omitempty"`
				Japanese      string `json:"japanese,omitempty"`
				Twitch        string `json:"twitch,omitempty"`
			} `json:"names,omitempty"`
			Abbreviation string   `json:"abbreviation,omitempty"`
			Platforms    []string `json:"platforms,omitempty"`
			Regions      []string `json:"regions,omitempty"`
			// im not sure this works
			Moderators []string `json:"moderators,omitempty"`
			Assets     struct {
				Trophy1st struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-1st,omitempty"`
				Trophy2nd struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-2nd,omitempty"`
				Trophy3rd struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-3rd,omitempty"`
				Trophy4th struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-4th,omitempty"`
			} `json:"assets,omitempty"`
		} `json:"data,omitempty"`
	} `json:"game,omitempty"`
	Level struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"level,omitempty"`
	Category struct {
		Data struct {
			ID            string `json:"id,omitempty"`
			Name          string `json:"name,omitempty"`
			Type          string `json:"type,omitempty"`
			Miscellaneous bool   `json:"miscellaneous,omitempty"`
			Variables     struct {
				Data []struct {
					ID       string      `json:"id,omitempty"`
					Name     string      `json:"name,omitempty"`
					Category interface{} `json:"category,omitempty"`
					Scope    struct {
						Type string `json:"type,omitempty"`
					} `json:"scope,omitempty"`
					Mandatory     bool                   `json:"mandatory,omitempty"`
					UserDefined   bool                   `json:"user-defined,omitempty"`
					Obsoletes     bool                   `json:"obsoletes,omitempty"`
					Values        map[string]interface{} `json:"values,omitempty"`
					IsSubcategory bool                   `json:"is-subcategory,omitempty"`
				} `json:"data,omitempty"`
			} `json:"variables,omitempty"`
		} `json:"data,omitempty"`
	} `json:"category,omitempty"`
	Times struct {
		Primary          float64 `json:"primary_t,omitempty"`
		RealTime         float64 `json:"realtime_t,omitempty"`
		RealTimeLoadless float64 `json:"realtime_noloads_t,omitempty"`
		InGameTime       float64 `json:"ingame_t,omitempty"`
	}
	Videos struct {
		Links []struct {
			URI string `json:"uri,omitempty"`
		} `json:"links,omitempty"`
	} `json:"videos,omitempty"`
	Comment string `json:"comment,omitempty"`
	Status  struct {
		Status     string    `json:"status,omitempty"`
		Examiner   string    `json:"examiner,omitempty"`
		Reason     string    `json:"reason,omitempty"`
		VerifyDate time.Time `json:"verify-date,omitempty"`
	} `json:"status,omitempty"`
	Players struct {
		Data []struct {
			ID      string `json:"id,omitempty"`
			Weblink string `json:"weblink,omitempty"`
			Names   struct {
				International string `json:"international,omitempty"`
				Japanese      string `json:"japanese,omitempty"`
			} `json:"names,omitempty"`
			Name   string `json:"name,omitempty"`
			Assets struct {
				Icon struct {
					URI string `json:"uri,omitempty"`
				} `json:"icon,omitempty"`
				Image struct {
					URI string `json:"uri,omitempty"`
				} `json:"image,omitempty"`
			} `json:"assets,omitempty"`
		} `json:"data,omitempty"`
	} `json:"players,omitempty"`
	Date      string    `json:"date,omitempty"`
	Submitted time.Time `json:"submitted,omitempty"`
	System    struct {
		Platform string `json:"platform,omitempty"`
		Emulated bool   `json:"emulated,omitempty"`
		Region   string `json:"region,omitempty"`
	} `json:"system,omitempty"`
	Values map[string]string `json:"values,omitempty"`
	Region struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"region,omitempty"`
	Platform struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"platform,omitempty"`
}

type PBs struct {
	Data []struct {
		Place int `json:"place,omitempty"`
		Run   struct {
			ID string `json:"id,omitempty"`
		} `json:"run,omitempty"`
	} `json:"data,omitempty"`
}

type Response struct {
	Data       []Data `json:"data,omitempty"`
	Pagination struct {
		Offset int `json:"offset,omitempty"`
		Max    int `json:"max,omitempty"`
		Size   int `json:"size,omitempty"`
	} `json:"pagination,omitempty"`
	Scope string `json:"scope,omitempty"`
}

func (c *SpeedrunClient) Do(req *http.Request) (*http.Response, error) {
	ctx := context.Background()
	err := c.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DiscordClient) Do(webhookUrl string, body []byte) (*http.Response, error) {
	ctx := context.Background()
	err := c.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}
	res, err := http.Post(webhookUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	return res, nil
}

func NewSClient(r *rate.Limiter) *SpeedrunClient {
	cl := http.Client{
		Timeout: time.Second * 10,
	}
	c := &SpeedrunClient{
		client:  &cl,
		limiter: r,
	}
	return c
}

func NewDClient(r *rate.Limiter) *DiscordClient {
	cl := http.Client{
		Timeout: time.Second * 10,
	}
	c := &DiscordClient{
		client:  &cl,
		limiter: r,
	}
	return c
}

func main() {
	listenAddr := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	rS, rD := rate.NewLimiter(rate.Every(1*time.Minute), 33), rate.NewLimiter(rate.Every(3*time.Second), 5)
	cS, cD = *NewSClient(rS), *NewDClient(rD)
	http.HandleFunc("/api/SendWebhook", runsHandler)
	log.Printf("About to listen on %s. Go to https://127.0.0.1%s/", listenAddr, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func runsHandler(w http.ResponseWriter, r *http.Request) {
	// todo: handle invalid bodies

	body := r.Body
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		log.Fatal(err)
	}
	var data []Data
	err = json.Unmarshal(b, &data)
	if err != nil {
		log.Fatal(err)
	}
	webhooks := GetWebhooks()
	switch data[0].Status.Status {
	case "verified":
		HandleVerified(&data, &webhooks)
	case "new":
		break
	case "rejected":
		break
	}
	w.WriteHeader(200)
}

func HandleVerified(data *[]Data, webhooks *[]Webhook) {
	wg.Add(1)
	for _, run := range *data {
		webhooksToSend := make(map[string]Webhook)
	nextWebhook:
		for _, webhook := range *webhooks {
			for _, category := range webhook.Records.Categories {
				if category == run.Category.Data.ID {
					// this is handling for a guest player as the first player, we don't want to send a webhook for that
					if run.Players.Data[0].Names.International != "" {
						webhook.PlayerIndex = 0
						webhooksToSend[webhook.WebhookUrl] = webhook
						continue nextWebhook
					}
				}
			}
			for i, player := range run.Players.Data {
				for _, wPlayer := range webhook.Records.Users {
					if wPlayer == player.ID {
						webhook.PlayerIndex = i
						webhooksToSend[webhook.WebhookUrl] = webhook
						continue nextWebhook
					}
				}
			}
		}
		if len(webhooksToSend) > 0 {
			wg.Add(1)
			go SendWebhook(&webhooksToSend, run, "verified")
		}
	}
	wg.Done()
}

func SendWebhook(webhooks *map[string]Webhook, run Data, scope string) {
	// todo: add handling for multiple webhooks wanting the same run
	switch scope {
	case "verified":
		var category, variables, players, iconUrl, runTime, place, author string
		subCats := make(map[string]string)
		for key, value := range run.Values {
			for _, variable := range run.Category.Data.Variables.Data {
				if variable.ID == key {
					for varId, varVal := range variable.Values["choices"].(map[string]interface{}) {
						if varId == value {
							if variable.IsSubcategory {
								subCats[key] = varId
								if category != "" {
									category += fmt.Sprintf(", %v", varVal)
								} else {
									category = fmt.Sprintf("%v (%v", run.Category.Data.Name, varVal)
								}
							} else {
								if variables != "" {
									variables += fmt.Sprintf(", %v: %v", variable.Name, varVal)
								} else {
									variables = fmt.Sprintf(" (%v: %v", variable.Name, varVal)
								}
							}
						}
					}
				}
			}
		}
		if len(run.Players.Data) > 1 {
			for _, player := range run.Players.Data {
				if players != "" {
					if player.Name == "" {
						players += fmt.Sprintf(", %v", player.Names.International)
					} else {
						players += fmt.Sprintf(", %v", player.Name)
					}
				} else {
					if player.Name == "" {
						players = player.Names.International
					} else {
						players = player.Name
					}
				}
			}
		}
		s, ms := math.Modf(run.Times.Primary)
		msString := fmt.Sprintf("%.3f", ms)
		dur := time.Duration(s) * time.Second
		runTime = dur.String()
		runTime = strings.Replace(runTime, "h", "h ", 1)
		runTime = strings.Replace(runTime, "m", "m ", 1)
		runTime = strings.Replace(runTime, "s", "s ", 1)
		if msString != "0.000" {
			runTime += msString[2:] + "ms"
		}
		if category != "" {
			category += ")"
		} else {
			category = run.Category.Data.Name
		}
		if variables != "" {
			variables += ")"
		}
		if runTime == "" {
			runTime = "0s"
		}
		fields := []map[string]interface{}{
			{
				"name":   "Category",
				"value":  category,
				"inline": true,
			},
			{
				"name":   "Time",
				"value":  runTime,
				"inline": true,
			},
		}
		if players != "" {
			fields = append([]map[string]interface{}{
				{
					"name":   "Players",
					"value":  players,
					"inline": true,
				},
			}, fields...)
		}
		if variables != "" {
			fields = append(fields, map[string]interface{}{
				"name":   "Variables",
				"value":  variables,
				"inline": true,
			})
		}
		if run.Level.Data.Name != "" {
			fields = append([]map[string]interface{}{
				{
					"name":   "Level",
					"value":  run.Level.Data.Name,
					"inline": true,
				},
			}, fields...)
		}
		userPBs := make(map[int]PBs)
		for _, webhook := range *webhooks {
			wg.Add(1)
			if _, ok := userPBs[webhook.PlayerIndex]; !ok {
				userPBs[webhook.PlayerIndex] = GetPersonalBests(run.Players.Data[webhook.PlayerIndex].ID, run.Game.Data.ID)
			}
			go func(webhook Webhook, playerIndex int) {
				// This is for making sure the proper user is being checked for sending the webhook
				isPb := false
				for _, pb := range userPBs[playerIndex].Data {
					if pb.Run.ID == run.ID {
						place = humanize.Ordinal(pb.Place)
						isPb = true
						break
					}
				}
				switch place {
				case "1st":
					if run.Game.Data.Assets.Trophy1st.URI != "" {
						iconUrl = run.Game.Data.Assets.Trophy1st.URI
					} else {
						iconUrl = "https://www.speedrun.com/images/1st.png"
					}
				case "2nd":
					if run.Game.Data.Assets.Trophy2nd.URI != "" {
						iconUrl = run.Game.Data.Assets.Trophy2nd.URI
					} else {
						iconUrl = "https://www.speedrun.com/images/2nd.png"
					}
				case "3rd":
					if run.Game.Data.Assets.Trophy3rd.URI != "" {
						iconUrl = run.Game.Data.Assets.Trophy3rd.URI
					} else {
						iconUrl = "https://www.speedrun.com/images/3rd.png"
					}
				default:
					iconUrl = ""
				}
				if webhook.Records.Events == "pb" && !isPb {
					return
				}
				if len(run.Players.Data) > 2 {
					author = fmt.Sprintf("%v and %v others", run.Players.Data[playerIndex].Names.International, len(run.Players.Data)-1)
				} else if len(run.Players.Data) > 1 {
					author = fmt.Sprintf("%v and %v other", run.Players.Data[playerIndex].Names.International, len(run.Players.Data)-1)
				} else {
					author = run.Players.Data[playerIndex].Names.International
				}
				embeds := []map[string]interface{}{
					{
						"author": map[string]interface{}{
							"name":     run.Players.Data[playerIndex].Names.International,
							"url":      run.Players.Data[playerIndex].Weblink,
							"icon_url": run.Players.Data[playerIndex].Assets.Image.URI,
						},
						"color":  "15899392",
						"fields": fields,
						"url":    run.Weblink,
					},
				}
				if place != "" && isPb {
					embeds[0]["footer"] = map[string]interface{}{
						"text":     fmt.Sprintf("They're now %v place!", place),
						"icon_url": iconUrl,
					}
				}
				if place == "1st" {
					embeds[0]["title"] = fmt.Sprintf("New world record in %v in %v!", category, run.Game.Data.Names.International)
					embeds[0]["description"] = fmt.Sprintf("**%v** got a new world record in **%v**!", author, run.Game.Data.Names.International)
				} else if isPb {
					embeds[0]["title"] = fmt.Sprintf("New personal best by %v!", author)
					embeds[0]["description"] = fmt.Sprintf("**%v** got a new personal best in **%v**!", author, run.Game.Data.Names.International)
				} else {
					embeds[0]["title"] = fmt.Sprintf("New run by %v!", author)
					embeds[0]["description"] = fmt.Sprintf("**%v** submitted a new run in **%v**!", author, run.Game.Data.Names.International)
				}
				jsonBody := map[string]interface{}{
					"content":     nil,
					"embeds":      embeds,
					"attachments": nil,
				}
				body, err := json.Marshal(jsonBody)
				if err != nil {
					log.Printf("Error while marshalling webhook body!\n%v", err)
				}
				res, err := cD.Do(webhook.WebhookUrl, body)
				if err != nil {
					log.Printf("Error while sending webhook!\n%v", err)
				}
				if res.StatusCode == 404 {
					log.Printf("Webhook %v is not found!", webhook.WebhookUrl)
					DeleteWebhook(webhook)
				}
				if res.StatusCode == 429 {
					log.Printf("Webhook %v is over rate limit!", webhook.WebhookUrl)
				}
				wg.Done()
			}(webhook, webhook.PlayerIndex)
		}
	case "new":
		break
	case "rejected":
		break
	}
	wg.Done()
}

func Connect() *mongo.Client {
	connectionString := os.Getenv(connectionStringEnv)
	if connectionString == "" {
		log.Fatal("The database connection string variable is missing!")
	}
	database = os.Getenv(databaseNameEnv)
	if database == "" {
		log.Fatal("The database name variable is missing!")
	}
	collection = "webhook-list"
	if collection == "" {
		log.Fatal("The collection name variable is missing!")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	options := options.Client().ApplyURI(connectionString).SetDirect(true)
	c, _ := mongo.NewClient(options)

	err := c.Connect(ctx)
	if err != nil {
		log.Fatalf("Connection couldn't be initialized!\n%v", err)
	}
	err = c.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Connection didn't respond back!\n%v", err)
	}
	return c
}

func GetWebhooks() []Webhook {
	c := Connect()
	defer c.Disconnect(context.TODO())
	collection := c.Database(database).Collection(collection)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var webhooks []Webhook
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		log.Fatalf("Error while finding webhooks!\n%v", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result Webhook
		err := cur.Decode(&result)
		if err != nil {
			log.Fatalf("Error while decoding webhooks!\n%v", err)
		}
		webhooks = append(webhooks, result)
	}
	if err := cur.Err(); err != nil {
		log.Fatalf("Error while iterating webhooks!\n%v", err)
	}
	return webhooks
}

func DeleteWebhook(webhook Webhook) {
	c := Connect()
	defer c.Disconnect(context.TODO())
	collection := c.Database(database).Collection(collection)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	result, err := collection.DeleteOne(ctx, bson.M{"WebhookUrl": webhook.WebhookUrl})
	if err != nil {
		log.Printf("Error while deleting webhook!\n%v", err)
	}
	log.Printf("Deleted %v webhook with url %v", result.DeletedCount, webhook.WebhookUrl)
}

func GetPersonalBests(user string, game string) PBs {
	queryFrag := fmt.Sprintf("/users/%v/personal-bests?game=%v&vary=%v", user, game, time.Now().Nanosecond())
	r, err := cS.Do(createReq(queryFrag))
	if err != nil {
		log.Printf("Error while getting leaderboard!\n%v", err)
	}
	return parseResPBs(r)
}

func createReq(queries string) *http.Request {
	url := "https://speedrun.com/api/v1"
	req, err := http.NewRequest("GET", url+queries, nil)
	if err != nil {
		log.Panic(err)
	}
	req.Header.Add("User-Agent", "SRCStats Webhook")
	return req
}

func parseResPBs(r *http.Response) PBs {
	if r == nil || r.StatusCode == 400 || r.StatusCode == 404 {
		return *new(PBs)
	}
	result, err := io.ReadAll(r.Body)
	if err != nil {
		log.Panic(err)
	}
	r.Body.Close()
	var res PBs
	json.Unmarshal(result, &res)
	return res
}
