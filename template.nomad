job "alr-updater" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"

  group "alr-updater" {
    network {
      port "webhook" {
        to = 8080
      }
    }

    volume "alr-updater-data" {
      type      = "host"
      source    = "alr-updater-data"
      read_only = false
    }

    task "alr-updater" {
      driver = "docker"

      volume_mount {
        volume      = "alr-updater-data"
        destination = "/etc/alr-updater"
        read_only   = false
      }

      env {
        GIT_REPO_DIR             = "/etc/alr-updater/repo"
        GIT_REPO_URL             = "https://gitea.plemya-x.ru/Plemya-x/alr-repo.git"
        GIT_CREDENTIALS_USERNAME = "alr-repo-bot"
        GIT_CREDENTIALS_PASSWORD = "${GITEA_PASSWORD}"
        GIT_COMMIT_NAME          = "alr-repo-bot"
        GIT_COMMIT_EMAIL         = "bot@plemya-x.ru"
        WEBHOOK_PASSWORD_HASH    = "${PASSWORD_HASH}"

        // Hack to force Nomad to re-deploy the service
        // instead of ignoring it
        COMMIT_SHA = "${DRONE_COMMIT_SHA}"
      }

      config {
        image   = "alpine:latest"
        command = "/opt/alr-updater/alr-updater"
        args    = ["-DE"]
        ports   = ["webhook"]
        volumes = ["local/alr-updater/:/opt/alr-updater:ro"]
      }

      artifact {
        source      = "https://gitea.plemya-x.ru/api/v1/repos/Plemya-x/ALR-updater/releases/latest/alr-updater-$${attr.cpu.arch}.tar.gz"
        destination = "local/alr-updater"
      }

      service {
        name = "alr-updater"
        port = "webhook"

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.alr-updater.rule=Host(`updater.plemya-x.ru`)",
          "traefik.http.routers.alr-updater.tls.certResolver=letsencrypt",
        ]
      }
    }
  }
}

