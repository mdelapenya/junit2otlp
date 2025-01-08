package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mdelapenya/junit2otlp/internal/config"
	"github.com/mdelapenya/junit2otlp/internal/otel"
	"github.com/mdelapenya/junit2otlp/internal/readers"
	"github.com/mdelapenya/junit2otlp/internal/scm"
	"github.com/mdelapenya/junit2otlp/internal/transform"
)

func main() {
	cfg, err := config.NewConfigFromArgs()
	if err != nil {
		log.Fatalf("failed to prepare config: %s", err)
	}

	ctx := context.Background()
	err = Run(ctx, cfg, &readers.PipeReader{})
	if err != nil {
		log.Fatal(err)
	}
}

func Run(ctx context.Context, cfg *config.Config, reader readers.InputReader) error {
	suites, err := readers.ReadJUnitReport(reader)
	if err != nil {
		return fmt.Errorf("failed to read JUnit report: %w", err)
	}

	// setup the otel otelProvider
	otelProvider, err := otel.NewProvider(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create otel provider: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()
		// pushes any last exports to the receiver
		otelProvider.Shutdown(ctx)
	}()

	var runtimeAttributes = otel.RuntimeAttributes()

	// read the repo and get the attributes
	repo := scm.GetScm(cfg.RepositoryPath)
	if repo != nil {
		scmAttributes := repo.ContributeAttributes()
		runtimeAttributes = append(runtimeAttributes, scmAttributes...)
	}

	// add additional attributes to the runtime attributes
	runtimeAttributes = append(runtimeAttributes, cfg.AdditionalAttributes...)

	// transform and load the JUnit report into OTLP
	err = transform.TransformAndLoadSuites(ctx, cfg, otelProvider, suites, runtimeAttributes)
	if err != nil {
		return err
	}

	return nil
}
