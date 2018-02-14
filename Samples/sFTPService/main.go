package main

//implements the SFTP Service. Needs to be installed and managed using NSSM.

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"
)

type configuration struct {
	Source      string
	Host        string
	Port        int
	Username    string
	Password    string
	Destination string
}

func main() {
	//listen on port 8084
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8084", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	conf := configuration{}
	conf.Source = r.FormValue("Source")
	conf.Username = r.FormValue("Username")
	conf.Password = r.FormValue("Password")
	conf.Host = r.FormValue("Host")
	port, err := strconv.Atoi(r.FormValue("Port"))

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conf.Port = port
	conf.Destination = r.FormValue("Folder")

	err = uploadFiles(conf)

	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func getConfiguration(confString string) (configuration, error) {
	conf := configuration{}
	err := json.Unmarshal([]byte(confString), &conf)

	if err != nil {
		return conf, err
	}

	return conf, nil
}

func uploadFiles(conf configuration) error {
	//init sftp client
	var authMethods []ssh.AuthMethod

	keyboardInteractiveChallenge := func(
		user,
		instruction string,
		questions []string,
		echos []bool,
	) (answers []string, err error) {
		if len(questions) == 0 {
			return []string{}, nil
		}
		return []string{conf.Password}, nil
	}
	//add KeyboardInteractive and Password auth methods
	authMethods = append(authMethods, ssh.KeyboardInteractive(keyboardInteractiveChallenge))
	authMethods = append(authMethods, ssh.Password(conf.Password))

	config := &ssh.ClientConfig{
		User: conf.Username,
		Auth: authMethods,
	}

	//get source folder
	files, err := ioutil.ReadDir(conf.Source)
	if err != nil {
		log.Fatal(err)
	}

	if len(files) > 0 {
		//open sftp connection
		client, err := ssh.Dial("tcp", conf.Host+":"+strconv.Itoa(conf.Port), config)
		if err != nil {
			log.Fatal(err)
		}

		sftp, err := sftp.NewClient(client)
		if err != nil {
			log.Fatal(err)
		}
		defer sftp.Close()

		var successfiles []string

		for _, f := range files {
			filename := f.Name()
			if strings.HasSuffix(filename, ".txt") {
				sourcePath := conf.Source + "\\" + filename
				b, err := ioutil.ReadFile(sourcePath)
				if err != nil {
					log.Fatal(err)
				}

				//write file to server
				destPath := conf.Destination + "/" + filename
				f, err := sftp.Create(destPath)
				if err != nil {
					log.Fatal(err)
				}

				defer f.Close()
				f.Write(b)

				//add file to list of files that will be deleted
				successfiles = append(successfiles, sourcePath)
			}
		}

		//delete successfully uploaded files
		for _, s := range successfiles {
			os.Remove(s)
		}
	}
	return nil
}