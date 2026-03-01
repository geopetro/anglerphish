package main

/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/controllers"
	"github.com/gophish/gophish/crypto"
	"github.com/gophish/gophish/dialer"
	"github.com/gophish/gophish/imap"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/webhook"
)

// handleGenerateEncryptionKey generates a new encryption key and prints it
func handleGenerateEncryptionKey() {
	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		fmt.Printf("Error generating encryption key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s=%s\n", crypto.EncryptionKeyEnvVar, key)
}

// handleEncryptionStatus prints the current encryption status
func handleEncryptionStatus(conf *config.Config) {
	// Initialize encryption first
	enabled, err := crypto.InitEncryption()
	if err != nil {
		fmt.Printf("Error initializing encryption: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Encryption Enabled: %v\n", enabled)
	if !enabled {
		fmt.Printf("\nTo enable encryption, set the %s environment variable with a 32-character key.\n", crypto.EncryptionKeyEnvVar)
		fmt.Println("Generate a key with: ./gophish --generate-encryption-key")
	}

	// Get status from database
	report, err := models.GetEncryptionStatus()
	if err != nil {
		fmt.Printf("Error getting encryption status: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(report)
}

// handleMigrateEncryption encrypts all plaintext sensitive fields
func handleMigrateEncryption(conf *config.Config, isDryRun bool) {
	// Initialize encryption first
	enabled, err := crypto.InitEncryption()
	if err != nil {
		fmt.Printf("Error initializing encryption: %v\n", err)
		os.Exit(1)
	}

	if !enabled {
		fmt.Printf("Error: Encryption is not enabled. Set the %s environment variable first.\n", crypto.EncryptionKeyEnvVar)
		os.Exit(1)
	}

	if isDryRun {
		fmt.Println("DRY RUN - Previewing encryption migration (no changes will be made)...")
		results, err := models.DryRunMigrateToEncrypted()
		if err != nil {
			fmt.Printf("Error during dry run: %v\n", err)
			os.Exit(1)
		}
		for _, result := range results {
			fmt.Println(result)
		}
		fmt.Println("\nDry run complete. Use --migrate-encryption without --dry-run to apply changes.")
	} else {
		fmt.Println("Migrating plaintext fields to encrypted...")
		results, err := models.MigrateToEncrypted()
		if err != nil {
			fmt.Printf("Error during migration: %v\n", err)
			os.Exit(1)
		}
		for _, result := range results {
			fmt.Println(result)
		}
		fmt.Println("Migration complete!")
	}
}

// handleMigrateDecryption decrypts all encrypted fields back to plaintext
func handleMigrateDecryption(conf *config.Config, isDryRun bool) {
	// Initialize encryption first
	enabled, err := crypto.InitEncryption()
	if err != nil {
		fmt.Printf("Error initializing encryption: %v\n", err)
		os.Exit(1)
	}

	if !enabled {
		fmt.Printf("Error: Encryption is not enabled. Set the %s environment variable first.\n", crypto.EncryptionKeyEnvVar)
		os.Exit(1)
	}

	if isDryRun {
		fmt.Println("DRY RUN - Previewing decryption migration (no changes will be made)...")
		results, err := models.DryRunMigrateToDecrypted()
		if err != nil {
			fmt.Printf("Error during dry run: %v\n", err)
			os.Exit(1)
		}
		for _, result := range results {
			fmt.Println(result)
		}
		fmt.Println("\nDry run complete. Use --migrate-decryption without --dry-run to apply changes.")
	} else {
		fmt.Println("Decrypting all encrypted fields...")
		results, err := models.MigrateToDecrypted()
		if err != nil {
			fmt.Printf("Error during decryption: %v\n", err)
			os.Exit(1)
		}
		for _, result := range results {
			fmt.Println(result)
		}
		fmt.Println("Decryption complete!")
	}
}

const (
	modeAll   string = "all"
	modeAdmin string = "admin"
	modePhish string = "phish"
)

var (
	configPath            = kingpin.Flag("config", "Location of config.json.").Default("./config.json").String()
	disableMailer         = kingpin.Flag("disable-mailer", "Disable the mailer (for use with multi-system deployments)").Bool()
	mode                  = kingpin.Flag("mode", fmt.Sprintf("Run the binary in one of the modes (%s, %s or %s)", modeAll, modeAdmin, modePhish)).Default("all").Enum(modeAll, modeAdmin, modePhish)
	generateEncryptionKey = kingpin.Flag("generate-encryption-key", "Generate a new random encryption key and exit").Bool()
	migrateEncryption     = kingpin.Flag("migrate-encryption", "Encrypt all plaintext sensitive fields in the database and exit").Bool()
	migrateDecryption     = kingpin.Flag("migrate-decryption", "Decrypt all encrypted sensitive fields in the database and exit").Bool()
	showEncryptionStatus  = kingpin.Flag("encryption-status", "Show the encryption status of the database and exit").Bool()
	dryRun                = kingpin.Flag("dry-run", "Preview migration changes without applying them (use with --migrate-encryption or --migrate-decryption)").Bool()
)

func main() {
	// Load the VERSION file
	version, err := ioutil.ReadFile("./VERSION")
	if err != nil {
		log.Fatal("Could not read VERSION file: ", err)
	}
	kingpin.Version(string(version))

	// Load the ANGLERPHISH_VERSION file
	anglerphishVersion, err := ioutil.ReadFile("./ANGLERPHISH_VERSION")
	if err != nil {
		log.Fatal("Could not read ANGLERPHISH_VERSION file: ", err)
	}

	// Parse the CLI flags and load the config
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()

	// Handle encryption key generation (doesn't need config or database)
	if *generateEncryptionKey {
		handleGenerateEncryptionKey()
		return
	}

	// Load the config
	conf, err := config.LoadConfig(*configPath)
	// Just warn if a contact address hasn't been configured
	if err != nil {
		log.Fatal(err)
	}
	if conf.ContactAddress == "" {
		log.Warnf("No contact address has been configured.")
		log.Warnf("Please consider adding a contact_address entry in your config.json")
	}
	config.Version = string(version)
	config.AnglerPhishVersion = string(anglerphishVersion)

	// Configure our various upstream clients to make sure that we restrict
	// outbound connections as needed.
	dialer.SetAllowedHosts(conf.AdminConf.AllowedInternalHosts)
	webhook.SetTransport(&http.Transport{
		DialContext: dialer.Dialer().DialContext,
	})

	err = log.Setup(conf.Logging)
	if err != nil {
		log.Fatal(err)
	}

	// Provide the option to disable the built-in mailer
	// Setup the global variables and settings
	err = models.Setup(conf)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize encryption system
	encryptionEnabled, err := crypto.InitEncryption()
	if err != nil {
		log.Fatal("Error initializing encryption: ", err)
	}
	if encryptionEnabled {
		log.Info("Database encryption is enabled")
	}

	// Handle encryption CLI commands (these need database to be initialized)
	if *showEncryptionStatus {
		handleEncryptionStatus(conf)
		return
	}
	if *migrateEncryption {
		handleMigrateEncryption(conf, *dryRun)
		return
	}
	if *migrateDecryption {
		handleMigrateDecryption(conf, *dryRun)
		return
	}

	// Unlock any maillogs and smslogs that may have been locked for processing
	// when Gophish was last shutdown.
	err = models.UnlockAllMailLogs()
	if err != nil {
		log.Fatal(err)
	}

	err = models.UnlockAllSMSLogs()
	if err != nil {
		log.Fatal(err)
	}

	// Create our servers
	adminOptions := []controllers.AdminServerOption{}
	if *disableMailer {
		adminOptions = append(adminOptions, controllers.WithWorker(nil))
	}
	adminConfig := conf.AdminConf
	adminServer := controllers.NewAdminServer(adminConfig, conf, adminOptions...)
	middleware.Store.Options.Secure = adminConfig.UseTLS

	phishConfig := conf.PhishConf
	phishServer := controllers.NewPhishingServer(phishConfig)

	imapMonitor := imap.NewMonitor()
	if *mode == "admin" || *mode == "all" {
		go adminServer.Start()
		go imapMonitor.Start()
	}
	if *mode == "phish" || *mode == "all" {
		go phishServer.Start()
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info("CTRL+C Received... Gracefully shutting down servers")
	if *mode == modeAdmin || *mode == modeAll {
		adminServer.Shutdown()
		imapMonitor.Shutdown()
	}
	if *mode == modePhish || *mode == modeAll {
		phishServer.Shutdown()
	}

}
