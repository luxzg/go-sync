## Steps to setup 'sync-lite' development environment for modified Brave 'go-sync' server in VS Code (with Codex)
# All commands ran as user, except one sudo command to install Go to /usr/local
# After this is setup, query Codex through VS Code to rewrite to new folder "sync-lite"

1. **Install Go 1.26.0 (manual) + PATH**

# For download see: https://go.dev/dl/
# For install instructions see: https://go.dev/doc/install

```bash
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
# OR
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# check version
go version
```

2. **Install tools you used**

```bash
go install github.com/hexdigest/gowrap/cmd/gowrap@latest
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s --
```

3. **Create dev folder**

```bash
mkdir -p ~/dev/brave/
```

4. **Clone repo + basic checks**

```bash
cd ~/dev/brave/
git clone https://github.com/brave/go-sync.git
cd go-sync
git status
go mod tidy
```

5. **Open in VS Code**

```bash
code .
```

6. **Install VS Code extension**

* Install **Go** (by **Go Team at Google / go.dev**)

---

## 🧾 Summary checklist

* [x] Install Go 1.26.0 from tarball
* [x] Add `/usr/local/go/bin` to PATH and verify `go version`
* [x] Install `gowrap` and `golangci-lint`
* [x] Create `~/dev/brave/`
* [x] Clone `brave/go-sync`, run `git status`, `go mod tidy`
* [x] Open in VS Code (`code .`)
* [x] Install VS Code Go extension (Go Team at Google / go.dev)


## Steps to build the binary
After we have the code that works, passing tests and running, we will build the binary.

To do so:

```bash
cd ~/dev/brave/go-sync/sync-lite
go mod tidy
go build -o sync-lite .
```
