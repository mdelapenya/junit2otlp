define setup_demo_env
	$(eval ENV_FILE := demos/$(1)/demo.env)
	@echo ">> Setting up $(1) env from $(ENV_FILE)"
	$(eval include $(ENV_FILE))
	$(eval export $(shell sed 's/=.*//' $(ENV_FILE)))
	rm -fr demos/$(1)/build
endef

define start_demo
	mkdir demos/$(1)/build
	touch demos/$(1)/build/tests.json
	docker-compose -f demos/$(1)/docker-compose.yml up -d
	env | grep OTEL
endef

define stop_demo
	docker-compose -f demos/$(1)/docker-compose.yml down --remove-orphans --volumes
	rm -fr demos/$(1)/build
endef

build-docker-image:
	docker build -t mdelapenya/junit2otlp:latest .

demo-start-elastic:
	$(call setup_demo_env,elastic)
	$(call start_demo,elastic)

demo-stop-elastic:
	$(call stop_demo,elastic)

demo-start-jaeger:
	$(call setup_demo_env,jaeger)
	$(call start_demo,jaeger)

demo-stop-jaeger:
	$(call stop_demo,jaeger)

demo-start-prometheus:
	$(call setup_demo_env,prometheus)
	$(call start_demo,prometheus)

demo-stop-prometheus:
	$(call stop_demo,prometheus)

demo-start-zipkin:
	$(call setup_demo_env,zipkin)
	$(call start_demo,zipkin)

demo-stop-zipkin:
	$(call stop_demo,zipkin)
