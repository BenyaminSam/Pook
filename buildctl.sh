(
  set -eu

  VERSION=v0.33.0

  case "$(uname -m)" in
    x86_64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *)
      echo "Unsupported architecture: $(uname -m)"
      exit 1
      ;;
  esac

  WORKDIR="$(mktemp -d)"
  cd "$WORKDIR"

  curl -fL --retry 3 \
    "https://github.com/moby/buildkit/releases/download/${VERSION}/buildkit-${VERSION}.linux-${ARCH}.tar.gz" \
    -o buildkit.tar.gz

  tar -xzf buildkit.tar.gz

  sudo install -d /usr/local/bin
  sudo install -m 0755 bin/* /usr/local/bin/
)

