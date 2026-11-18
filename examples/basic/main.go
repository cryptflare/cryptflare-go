// Reveals a single secret using a service token. Run with:
//
//	CRYPTFLARE_SERVICE_TOKEN=cf_svc_... \
//	CRYPTFLARE_ORG=org_... \
//	CRYPTFLARE_WORKSPACE=my-app \
//	CRYPTFLARE_ENVIRONMENT=production \
//	go run ./examples/basic
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cryptflare/cryptflare-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := cryptflare.NewClient(
		cryptflare.WithDefaultOrganisation(os.Getenv("CRYPTFLARE_ORG")),
		cryptflare.WithDefaultWorkspace(os.Getenv("CRYPTFLARE_WORKSPACE")),
		cryptflare.WithDefaultEnvironment(os.Getenv("CRYPTFLARE_ENVIRONMENT")),
	)
	if err != nil {
		fail("construct client: %v", err)
	}

	page, err := client.Secrets.List(ctx, nil).First(ctx)
	if err != nil {
		fail("list secrets: %v", err)
	}
	for _, item := range page.Data {
		fmt.Printf("  %s  v%d  %s\n", item.Key, item.Version, item.UpdatedAt)
	}

	if len(page.Data) == 0 {
		fmt.Println("(no secrets in this environment)")
		return
	}

	revealed, err := client.Secrets.Reveal(ctx, &cryptflare.RevealSecretInput{Key: page.Data[0].Key})
	if err != nil {
		if errors.Is(err, cryptflare.ErrNotFound) {
			fail("expected the first listed secret to exist: %v", err)
		}
		fail("reveal: %v", err)
	}
	fmt.Printf("\nrevealed %s = %s\n", revealed.Key, revealed.Value)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
