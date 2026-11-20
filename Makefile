# mouse-runner 빌드 (macOS 전용, cgo + Xcode Command Line Tools 필요)
VERSION ?= dev
NAME    := mouse-runner
DIST    := dist
LDFLAGS := -s -w -X main.version=$(VERSION)
TARBALL := $(NAME)_$(VERSION)_darwin_universal.tar.gz

export CGO_ENABLED := 1
# Go 1.27 런타임이 macOS 13을 요구한다 (go.o가 13.0용으로 빌드됨)
export MACOSX_DEPLOYMENT_TARGET := 13.0

.PHONY: test build dist clean

test:
	go vet ./...
	go test ./...

# 이 맥용 실행 파일 ./mouse-runner
build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(NAME) .

# Apple Silicon + Intel 유니버설 바이너리와 배포용 압축 파일, 체크섬
dist: clean
	mkdir -p $(DIST)
	GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/$(NAME)-arm64 .
	GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/$(NAME)-amd64 .
	lipo -create -output $(DIST)/$(NAME) $(DIST)/$(NAME)-arm64 $(DIST)/$(NAME)-amd64
	rm $(DIST)/$(NAME)-arm64 $(DIST)/$(NAME)-amd64
	# 링커는 arm64 조각만 서명한다. 두 조각 모두 ad-hoc 서명을 붙인다.
	codesign --force --sign - $(DIST)/$(NAME)
	COPYFILE_DISABLE=1 tar -C $(DIST) -czf $(DIST)/$(TARBALL) $(NAME)
	cd $(DIST) && shasum -a 256 $(TARBALL) > checksums.txt

clean:
	rm -rf $(DIST) $(NAME)
