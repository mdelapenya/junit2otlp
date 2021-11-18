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
	echo '>> waiting for services...'
	env | grep OTEL
	sleep $(2)
	cat TEST-sample.xml | go run main.go semconv.go --service-name $(1) --service-version 1.0.0
	cat TEST-sample2.xml | go run main.go semconv.go --service-name $(1) --service-version 1.0.0
	cat TEST-sample3.xml | go run main.go semconv.go --service-name $(1) --service-version 1.0.0
endef

define stop_demo
	docker-compose -f demos/$(1)/docker-compose.yml down --remove-orphans --volumes
	rm -fr demos/$(1)/build
endef

demo-start-elastic:
	$(call setup_demo_env,elastic)
	$(call start_demo,elastic,5)

demo-stop-elastic:
	$(call stop_demo,elastic)
