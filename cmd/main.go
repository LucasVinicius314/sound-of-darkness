package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

var (
	CHANNEL_ID          = ""
	CLIENT_ID           = ""
	GUILD_ID            = ""
	TOKEN               = ""
	AUDIO_INPUT_FOLDER  = "../resources/audio/"
	AUDIO_OUTPUT_FOLDER = "../resources/encoded/"
	FILE_PATHS          = []string{}
	INTERVAL            = 1 * time.Minute
)

func main() {
	initEnv()

	filePaths, err := prepareSounds()
	if err != nil {
		log.Fatalf("failed to prepare sound: %v", err)
	}

	FILE_PATHS = filePaths

	client, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	client.AddHandler(ready)

	err = client.Open()
	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	client.Close()
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func getRandomItem(arr []string) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return arr[rand.Intn(len(arr))]
}

func initEnv() {
	CHANNEL_ID = os.Getenv("CHANNEL_ID")
	CLIENT_ID = os.Getenv("CLIENT_ID")
	GUILD_ID = os.Getenv("GUILD_ID")
	TOKEN = os.Getenv("TOKEN")
}

func prepareSounds() ([]string, error) {
	options := dca.StdEncodeOptions
	options.RawOutput = true
	options.Bitrate = 96

	files, err := os.ReadDir(AUDIO_INPUT_FOLDER)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			outPath := filepath.Join(AUDIO_OUTPUT_FOLDER, fmt.Sprintf("%s.dca", file.Name()))

			if fileExists(outPath) {
				log.Printf("skipped [%s]", file.Name())
				continue
			}

			source, err := dca.EncodeFile(filepath.Join(AUDIO_INPUT_FOLDER, file.Name()), options)
			if err != nil {
				return nil, fmt.Errorf("failed to encode file: %v", err)
			}
			defer source.Cleanup()

			output, err := os.Create(outPath)
			if err != nil {
				return nil, err
			}

			_, err = io.Copy(output, source)
			if err != nil {
				return nil, err
			}

			log.Printf("encoded [%s]", file.Name())
		}
	}

	outFiles, err := os.ReadDir(AUDIO_OUTPUT_FOLDER)
	if err != nil {
		return nil, err
	}

	filePaths := []string{}
	for _, file := range outFiles {
		filePaths = append(filePaths, filepath.Join(AUDIO_OUTPUT_FOLDER, file.Name()))
	}

	return filePaths, err
}

func playSound(vc *discordgo.VoiceConnection, filePath string) error {
	options := dca.StdEncodeOptions
	options.RawOutput = true
	options.Bitrate = 96

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	done := make(chan error)
	dca.NewStream(dca.NewDecoder(file), vc, done)

	err = <-done
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to close stream: %v", err)
	}

	return nil
}

// TODO: Logica para pegar o id do canal com maior quantidade de membros em determinada
// categoria do discord e fazer dar join nesse canal
func indentifyActiveChannel(s *discordgo.Session) (err error) {
	return CHANNEL_ID
}

func joinChannel(s *discordgo.Session, fileName string) (err error) {
	log.Printf("playing [%s]", fileName)

	activeChannelID := indentifyActiveChannel()

	vc, err := s.ChannelVoiceJoin(activeChannelID, false, true)
	if err != nil {
		return err
	}

	time.Sleep(250 * time.Millisecond)

	vc.Speaking(true)

	err = playSound(vc, fileName)
	if err != nil {
		return err
	}

	vc.Speaking(false)

	time.Sleep(250 * time.Millisecond)

	vc.Disconnect()

	return nil
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	s.UpdateGameStatus(0, "😈")

	setInterval(INTERVAL, func() {
		err := joinChannel(s, getRandomItem(FILE_PATHS))
		if err != nil {
			log.Printf("failed to join channel: %v", err)
		}
	})

	err := joinChannel(s, getRandomItem(FILE_PATHS))
	if err != nil {
		log.Printf("failed to join channel: %v", err)
	}
}

func setInterval(interval time.Duration, task func()) chan bool {
	ticker := time.NewTicker(interval)
	stop := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				task()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()

	return stop
}
