# Deployment (Gitea Actions → host)

Tanabata is deployed by a [Gitea Actions](https://docs.gitea.com/usage/actions/overview)
workflow ([`.gitea/workflows/deploy.yml`](../.gitea/workflows/deploy.yml)) that
runs on the **production host itself**. On every push to `master` it updates the
git clone in `/opt/tanabata`, runs the test suite (backend + frontend, in
throwaway toolchain containers), and — only if it passes — runs
`docker compose up -d --build` there, so the image is built from the
freshly-pushed code and the stack is restarted.

```
push master ──> Gitea (container) ──> act_runner (host, "host" label)
                                          │ git fetch + reset --hard   (in /opt/tanabata)
                                          │ run tests (go + node in containers; ephemeral Postgres)
                                          └ docker compose up -d --build   (only if tests pass)
```

The Gitea server runs in a container, but the **runner runs directly on the host**
(shell executor) so it can use the host's git, the host Docker daemon, and the
clone in `/opt/tanabata`. Nothing needs a registry — the host builds the image
locally. The workflow uses only shell steps, so the host needs just **git** and
**docker** (no node, no rsync).

## What is a runner?

Gitea (like GitHub) only *coordinates* CI: it stores the workflow, queues jobs,
and shows logs. It does **not** execute anything itself. A **runner** is a
separate agent program that polls Gitea for queued jobs, runs the steps on a
machine you control, and reports results back.

Gitea's official runner is **act_runner** (a single Go binary; it uses the
`act` engine to interpret workflow YAML). One act_runner process can serve many
repos. Each runner advertises one or more **labels**, and a job's `runs-on:`
picks a runner by label. A label also decides *how* a job runs — the **executor**:

- **docker executor** — each job runs in a fresh container from an image (e.g.
  `node:20-bookworm`). Isolated and reproducible; the usual default. Label form
  at registration: `ubuntu:docker://node:20-bookworm`.
- **host / shell executor** — the job runs directly on the host as the runner's
  user, using host-installed tools. Label form: `host:host`. This is what we use,
  because the deploy needs the host's Docker daemon and `/opt/tanabata`.

So `runs-on: host` in the workflow ⇒ "run this job on a runner that registered a
`host` label" ⇒ our shell executor on the prod box.

## One-time setup

### 1. Enable Actions in Gitea

Gitea 1.21+ has Actions on by default. Otherwise add to `app.ini` and restart:

```ini
[actions]
ENABLED = true
```

### 2. A runner user on the host

Pick (or create) the Linux user the runner runs as. It must be able to use Docker
and own the deploy dir — so the workflow needs no `sudo`:

```bash
sudo useradd -r -m -d /home/gitea-runner gitea-runner   # or reuse an existing user
sudo usermod -aG docker gitea-runner                     # host Docker access
```

The host needs `git` and a Docker engine with the Compose plugin:

```bash
sudo apt install -y git docker.io docker-compose-plugin   # Debian/Ubuntu
```

### 3. Clone the repo to /opt/tanabata once

The workflow only does `git fetch` + `reset --hard`, so the clone (and its auth)
is established here, once. Use a **read-only deploy key** so the host never holds
write credentials:

```bash
# As the runner user, create a key and add the PUBLIC half to the repo in Gitea:
#   Repo → Settings → Deploy Keys → Add (read-only)
sudo -u gitea-runner ssh-keygen -t ed25519 -f /home/gitea-runner/.ssh/tanabata_deploy -N ''

# Clone with that key (SSH URL of your Gitea repo):
sudo -u gitea-runner GIT_SSH_COMMAND='ssh -i /home/gitea-runner/.ssh/tanabata_deploy' \
  git clone git@gitea.example.com:you/tanabata.git /opt/tanabata
sudo chown -R gitea-runner:gitea-runner /opt/tanabata
```

> HTTPS works too — clone with a URL that carries a read-only token. SSH deploy
> keys are the cleaner, per-repo, read-only option.

After cloning, recurring `git fetch` reuses the remote + key stored in
`/opt/tanabata/.git/config`, so the runner itself needs no standing credentials.

### 4. Register and run act_runner on the host

Get a registration token in Gitea. **Where you create it sets the runner's
scope** (and `--name` is only a display label, unrelated to scope):

- **Repository** (Tanabata repo → Settings → Actions → Runners) → serves only
  this repo. **Use this.**
- Organization → all repos in the org; Site (admin) → all repos on the instance.

> Security: this runner is a host/shell executor with access to the Docker
> socket — effectively root on the host. Register it at the **repository** level
> so only Tanabata's workflows can run on your prod server; a site-wide runner
> would let any repo's workflow execute arbitrary commands here.

Then, as the runner user:

```bash
# Download act_runner: https://gitea.com/gitea/act_runner/releases
act_runner register --no-interactive \
  --instance https://gitea.example.com \
  --token   <REGISTRATION_TOKEN> \
  --name    prod-host \
  --labels  host:host          # <-- maps `runs-on: host` to the shell executor

# Run it (use a systemd unit in production so it survives reboots):
act_runner daemon
```

`--labels host:host` is what makes jobs run **on the host** instead of in a
container. The instance URL must be reachable from the host (Gitea's published
port / domain — not the in-container address). Registration writes a `.runner`
file (the runner's credentials) in the working directory.

Minimal systemd unit (`/etc/systemd/system/act_runner.service`):

```ini
[Unit]
Description=Gitea act_runner
After=docker.service
Requires=docker.service

[Service]
User=gitea-runner
WorkingDirectory=/home/gitea-runner
ExecStart=/usr/local/bin/act_runner daemon
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now act_runner
```

### 5. Create /opt/tanabata/.env (secrets)

The workflow **never** writes `.env` — it lives on the host and holds the real
secrets and the chosen DB mode. `.env` is git-ignored, so `git reset --hard`
leaves it untouched. Create it once:

```bash
cd /opt/tanabata
sudo -u gitea-runner cp .env.example .env
sudo -u gitea-runner $EDITOR .env    # set JWT_SECRET, ADMIN_PASSWORD, DATABASE_URL, etc.
```

See [`.env.example`](../.env.example) for every variable. For the bundled
Postgres keep `COMPOSE_PROFILES=with-db`; to use a Postgres already on the host,
set it empty and point `DATABASE_URL` at `host.docker.internal`.

> Data lives in named Docker volumes by default (or the `*_DIR` host paths you
> set in `.env`, e.g. `/var/lib/tanabata/...`) — **not** in `/opt/tanabata`. So
> `git reset --hard` on the code dir never touches your data.

## Deploying

Push to `master` (or hit **Run workflow** on the Actions tab). Watch progress
under the repo's **Actions** tab. The first build pulls the Node/Go base images
and takes a few minutes; later builds reuse the host's layer cache.

## Notes / alternatives

- **Docker-executor runner instead of host.** If you'd rather the runner itself
  run in a container, register with a Docker label and bind-mount
  `/var/run/docker.sock` and `/opt/tanabata` into the job (act_runner
  `config.yaml` → `container.valid_volumes`), then change `runs-on` accordingly.
  The host executor above is simpler for host deploys.
- **Zero-downtime** isn't attempted: `compose up` recreates changed containers.
  For a single-node setup the brief restart is usually fine.
