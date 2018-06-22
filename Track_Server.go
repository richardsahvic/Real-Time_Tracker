package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/centrifugal/centrifuge-go"
	"github.com/centrifugal/centrifugo/libcentrifugo/auth"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
)

type RegResponse struct {
	ClientID  	string `json:"id"`
	User 		string `json:"user"`
	Timestamp 	string `json:"timestamp"`
	Info 		string `json:"info"`
	Token 		string `json:"token"`
}

type LocRequest struct {
	LatCoordinate float64 `json:"reqlat"`
	LngCoordinate float64 `json:"reqlng"`
	ClientID    string  `json:"reqid"`
}

type LocResponse struct {
	Message string `json:"message"`
}

type ClientLocation struct {
	LatCoordinate float64 `json:"publat"`
	LngCoordinate float64 `json:"publng"`
	ClientID    string  `json:"pubid"`
}

//=============================
//CHANNEL
var forPublish = make(chan ClientLocation)
var secrKey = "555333ee-7f13-4aa4-b9e8-c3c1c64a48b9"
var info = ""
var token = ""
var user = ""
var timestamp = ""

func main() {

	//=============================
	// CENTRIFUGO
	secret := secrKey
	user = "001"
	timestamp = centrifuge.Timestamp()
	info = ""

	token = auth.GenerateClientToken(secret, user, timestamp, info)

	creds := &centrifuge.Credentials{
		User:      user,
		Timestamp: timestamp,
		Info:      info,
		Token:     token,
	}

	wsURL := "ws://localhost:8001/connection/websocket"
	conf := centrifuge.DefaultConfig
	c := centrifuge.NewCentrifuge(wsURL, creds, nil, conf)
	defer c.Close()

	err := c.Connect()
	if err != nil {
		log.Fatalln(err)
	}

	onJoin := func(sub centrifuge.Sub, msg centrifuge.ClientInfo) error {
		log.Println(fmt.Sprintf("User %s joined channel %s with client ID %s", msg.User, sub.Channel(), msg.Client))
		return nil
	}

	onLeave := func(sub centrifuge.Sub, msg centrifuge.ClientInfo) error {
		log.Println(fmt.Sprintf("User %s with clientID left channel %s with client ID %s", msg.User, msg.Client, sub.Channel()))
		return nil
	}

	events := &centrifuge.SubEventHandler{
		OnJoin:  onJoin,
		OnLeave: onLeave,
	}

	sub, err := c.Subscribe("public:track", events)
	if err != nil {
		log.Fatalln(err)
	}

	//=============================
	//NET/HTTP
	route := mux.NewRouter()
	route.HandleFunc("/getRegister", Register).
		Methods("GET")
	route.HandleFunc("/postLocation", Locate).
		Methods("POST")

	go handleCoordinate(sub)

	log.Println("SERVER STARTED")
	http.ListenAndServe(":8080", route)
}

//=============================
//GET
func Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	ID := uuid.Must(uuid.NewV4())

	regResp := RegResponse{
		ClientID:  ID.String(),
		Timestamp: timestamp,
		User: user,
		Info: info,
		Token: token,
	}
	json.NewEncoder(w).Encode(regResp)
}

//=============================
//POST
func Locate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	body, _ := ioutil.ReadAll(io.LimitReader(r.Body, 5000))

	var temp1 LocRequest
	json.Unmarshal(body, &temp1)

	temp2 := ClientLocation{
		LatCoordinate: 	temp1.LatCoordinate,
		LngCoordinate: 	temp1.LngCoordinate,
		ClientID:		temp1.ClientID,
	}
	forPublish <- temp2

	Reply := LocResponse{
		Message: "Coordinate recieved.",
	}
	json.NewEncoder(w).Encode(Reply)
}

//=============================
//GOROUTINE
func handleCoordinate(subscribedClient centrifuge.Sub) {
	for {
		data := <-forPublish
		Coor, _ := json.Marshal(data)
		err := subscribedClient.Publish(Coor)
		if err != nil {
			log.Fatalln(err)
		}
	}

}
