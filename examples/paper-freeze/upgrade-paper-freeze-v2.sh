#!/usr/bin/env bash
# ==============================================================================
#  paper-freeze v2 — instala do zero ou atualiza um v1.
#
#  v2 = credenciais AWS via `secrets` (kdbx-secrets, credential_process do AWS
#  CLI, nada em texto puro no rc) + snapshot com `git archive HEAD` (só o que
#  está commitado) em vez de zipar a pasta inteira.
#
#  Cenários:
#    - sem paper-freeze: pergunta bucket/prefixo/região e instala no rc do $SHELL;
#    - com v1 (ou v2):   reaproveita bucket/prefixo/região do bloco existente e
#                        o substitui pelo bloco v2 (backup 600 do rc antes).
#
#  Pré-requisitos: aws CLI, git, keepassxc-cli (ou KEEPASSXC_CLI) e `secrets` configurado (KDBX_SECRETS_HOME
#  exportado, se não usar ~/.kdbx-secrets). Suporta bash e zsh (não fish).
#
#  Ordem: template → profile no ~/.aws/config → Entry no KeePass (adapter fill) → teste do
#  profile → só então o rc é alterado. Rodar de novo é seguro.
# ==============================================================================
set -euo pipefail

PROFILE=paper-freeze
TEMPLATE=aws-paper-freeze
ENTRY=AWS/paper-freeze
MARKER_RE='^# paper-freeze( v2)? — added by installer'
AWS_CONFIG="${AWS_CONFIG_FILE:-$HOME/.aws/config}"

say() { printf '\033[0;36m\033[1m[paper-freeze v2]\033[0m %s\n' "$*"; }
die() { printf '\033[0;31m\033[1m  ✗\033[0m %s\n' "$*" >&2; exit 1; }
ok()  { printf '\033[0;32m\033[1m  ✓\033[0m %s\n' "$*"; }

# ask <var> <rótulo> [default] — lê do terminal; vazio sem default repete.
ask() {
  local v=""
  while :; do
    read -rp "  ? $2${3:+ [$3]}: " v < /dev/tty
    v="${v:-${3:-}}"
    [[ -n "$v" || "$1" == PREFIX ]] && break
  done
  printf -v "$1" '%s' "$v"
}

# rc_get <rc> <VAR> — valor de `export VAR="..."` dentro do rc (sem source).
rc_get() { sed -nE "s/^export $2=\"?([^\"]*)\"?$/\\1/p" "$1" 2>/dev/null | tail -1; }

# ── 0. Pré-checagens ───────────────────────────────────────────────────────────
SECRETS="$(command -v secrets || command -v kdbx-secrets)" || die "secrets (kdbx-secrets) não encontrado no PATH."
command -v aws >/dev/null || die "AWS CLI não encontrado."
command -v git >/dev/null || die "git não encontrado."
"$SECRETS" adapter list >/dev/null || die "secrets não está configurado (veja KDBX_SECRETS_HOME)."

RC_FILES=()
for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
  grep -qE "$MARKER_RE" "$rc" 2>/dev/null && RC_FILES+=("$rc")
done

if ((${#RC_FILES[@]})); then
  say "paper-freeze encontrado em: ${RC_FILES[*]} — atualizando."
  SRC="${RC_FILES[0]}"
  BUCKET="$(rc_get "$SRC" PAPER_FREEZE_S3_BUCKET)"
  PREFIX="$(rc_get "$SRC" PAPER_FREEZE_S3_PREFIX)"
  REGION="$(rc_get "$SRC" PAPER_FREEZE_AWS_REGION)"
else
  say "paper-freeze não encontrado — instalação nova."
  case "$(basename "${SHELL:-bash}")" in
    zsh) RC_FILES=("$HOME/.zshrc") ;;
    *)   RC_FILES=("$HOME/.bashrc") ;;
  esac
  BUCKET="" PREFIX="" REGION="" FRESH=1
fi
REGION="${REGION:-$(aws configure get region --profile "$PROFILE" 2>/dev/null || true)}"

[[ -n "$BUCKET" ]] || { echo "  O bucket precisa existir; o paper-freeze não o cria."; ask BUCKET "S3 bucket"; }
[[ -z "${FRESH:-}" ]] || ask PREFIX "S3 prefix (vazio = raiz do bucket)" ""
[[ -n "$REGION" ]] || ask REGION "AWS region" "us-east-1"

# ── 1. Template ────────────────────────────────────────────────────────────────
if "$SECRETS" adapter list | grep -qx "$TEMPLATE"; then
  ok "template $TEMPLATE já existe"
else
  printf '%s\n' "{\"Version\":1,\"AccessKeyId\":{{ json (get \"$ENTRY#UserName\") }},\"SecretAccessKey\":{{ json (get \"$ENTRY\") }}}" |
    "$SECRETS" adapter create "$TEMPLATE"
  ok "template $TEMPLATE criado"
fi

# ── 2. Profile no ~/.aws/config ────────────────────────────────────────────────
mkdir -p "$(dirname "$AWS_CONFIG")"
if grep -qx "\[profile $PROFILE\]" "$AWS_CONFIG" 2>/dev/null; then
  ok "profile $PROFILE já existe em $AWS_CONFIG"
else
  printf '\n[profile %s]\ncredential_process = %s render %s\nregion = %s\n' \
    "$PROFILE" "$SECRETS" "$TEMPLATE" "$REGION" >> "$AWS_CONFIG"
  ok "profile $PROFILE adicionado em $AWS_CONFIG"
fi

# ── 3. Entry no KeePass (o próprio secrets grava o que faltar) ─────────────────
echo
say "Entry $ENTRY: UserName = Access Key ID, Password = Secret Access Key."
grep -qs '^export PAPER_FREEZE_AWS_SECRET=' "${RC_FILES[@]}" &&
  echo "    (os valores atuais estão em PAPER_FREEZE_AWS_KEY / PAPER_FREEZE_AWS_SECRET no rc)"
echo "    O secrets pede a senha do banco e só os valores que faltarem; nada existente é alterado."
echo "    Feche o banco no KeePassXC se ele tiver alterações não salvas."
"$SECRETS" adapter fill "$TEMPLATE" ||
  die "não consegui gravar a Entry $ENTRY; o rc não foi alterado."

# ── 4. Teste — só segue se o profile autenticar ────────────────────────────────
say "Testando o profile (o secrets vai pedir a senha do banco)..."
NOENV=(env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN)
ARN="$("${NOENV[@]}" aws sts get-caller-identity --profile "$PROFILE" --query Arn --output text)" ||
  die "o profile $PROFILE não autenticou; o rc não foi alterado."
ok "autenticado como $ARN"
"${NOENV[@]}" aws s3 ls "s3://$BUCKET" --profile "$PROFILE" >/dev/null 2>&1 ||
  echo "  ⚠  não consegui listar s3://$BUCKET (confira se existe e se há s3:PutObject)."

# ── 5. Bloco v2 no rc ──────────────────────────────────────────────────────────
block() {
  echo '# ============================================================'
  echo '# paper-freeze v2 — added by installer'
  echo "# credenciais via secrets (profile $PROFILE no ~/.aws/config); snapshot via git archive"
  echo '# ============================================================'
  printf 'export PAPER_FREEZE_S3_BUCKET=%q\n' "$BUCKET"
  printf 'export PAPER_FREEZE_S3_PREFIX=%q\n' "$PREFIX"
  printf 'export PAPER_FREEZE_AWS_PROFILE=%q\n' "$PROFILE"
  cat <<'EOF'

# Wrap 'git' to show a paper-freeze reminder after every 'git push'
git() {
  command git "$@"
  local _exit=$?
  if [[ "$1" == "push" ]]; then
    echo ""
    echo "  🧊 Want to freeze the current state of your repository? use paper-freeze"
    echo ""
  fi
  return $_exit
}

# paper-freeze: snapshot do HEAD (git archive) enviado ao S3 (GLACIER)
paper-freeze() {
  local REPO_ROOT
  REPO_ROOT="$(command git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "❌  Not inside a git repository."
    return 1
  }
  command -v aws &>/dev/null || { echo "❌  AWS CLI not found. Please install it first."; return 1; }

  local REPO_NAME="$(basename "$REPO_ROOT")"
  local REV="$(command git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)" || {
    echo "❌  Repository has no commits."
    return 1
  }
  local ARCHIVE_NAME="${REPO_NAME}-$(date +%Y%m%d-%H%M%S)-${REV}.zip"
  local TMP_DIR="$(mktemp -d)"
  local ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_NAME}"
  local S3_KEY="${PAPER_FREEZE_S3_PREFIX:+${PAPER_FREEZE_S3_PREFIX}/}${ARCHIVE_NAME}"

  echo ""
  echo "🧊  paper-freeze"
  echo "    Repo   : $REPO_ROOT (HEAD $REV)"
  echo "    Archive: $ARCHIVE_NAME"
  echo "    Target : s3://${PAPER_FREEZE_S3_BUCKET}/${S3_KEY} (GLACIER)"
  [[ -n "$(command git -C "$REPO_ROOT" status --porcelain)" ]] &&
    echo "    ⚠  Uncommitted changes are NOT included (only HEAD is archived)."
  echo ""

  printf "    Proceed? [y/N] → "
  read -r _confirm < /dev/tty
  if [[ ! "$_confirm" =~ ^[Yy]$ ]]; then
    echo "    Aborted."
    rm -rf "$TMP_DIR"
    return 0
  fi

  echo ""
  echo "  ⏳ Archiving HEAD of $REPO_ROOT ..."
  command git -C "$REPO_ROOT" archive --format=zip --prefix="$REPO_NAME/" -o "$ARCHIVE_PATH" HEAD || {
    echo "❌  Failed to create archive."
    rm -rf "$TMP_DIR"
    return 1
  }
  echo "  ✓  Archive created ($(du -sh "$ARCHIVE_PATH" | cut -f1))"
  echo ""
  echo "  ⏳ Uploading to S3 ..."

  if aws --profile "${PAPER_FREEZE_AWS_PROFILE:-paper-freeze}" s3 cp "$ARCHIVE_PATH" \
      "s3://${PAPER_FREEZE_S3_BUCKET}/${S3_KEY}" --storage-class GLACIER --no-progress; then
    echo "  ✓  Upload complete!"
    echo ""
    echo "  📦 Frozen snapshot stored at:"
    echo "     s3://${PAPER_FREEZE_S3_BUCKET}/${S3_KEY}"
    echo ""
  else
    echo "❌  Upload failed. Check the '${PAPER_FREEZE_AWS_PROFILE:-paper-freeze}' profile and bucket permissions."
    rm -rf "$TMP_DIR"
    return 1
  fi

  rm -rf "$TMP_DIR"
  echo "  🧊 Repository frozen successfully. Stay cool. ✌️"
  echo ""
}
# ============================================================
# end paper-freeze
# ============================================================
EOF
}

# Remove o bloco antigo (v1 ou v2): da régua antes do marcador até a régua
# depois de "# end paper-freeze".
strip_block() {
  awk -v re="$MARKER_RE" '
    skip == 2 { skip = 0; next }                 # régua final
    skip      { if ($0 == "# end paper-freeze") skip = 2; next }
    $0 ~ re   { held = ""; skip = 1; next }      # descarta a régua inicial
    { if (has) print held; held = $0; has = 1 }
    END { if (has && !skip) print held }
  ' "$1"
}

for rc in "${RC_FILES[@]}"; do
  touch "$rc"
  if grep -qE "$MARKER_RE" "$rc"; then
    bak="$rc.paper-freeze-v2.bak"
    (umask 077 && cp "$rc" "$bak")     # pode conter o segredo do v1: 600
    say "backup do rc em $bak"
  fi
  tmp="$(mktemp "$rc.XXXX")"
  { strip_block "$rc"; echo; block; } > "$tmp"
  chmod --reference="$rc" "$tmp" 2>/dev/null || true
  mv "$tmp" "$rc"
  grep -q '^export PAPER_FREEZE_AWS_SECRET=' "$rc" && die "sobrou PAPER_FREEZE_AWS_SECRET em $rc; restaure do backup."
  ok "$rc com paper-freeze v2"
done

echo
say "Pronto. Próximos passos:"
echo "    1. abra um shell novo (exec \$SHELL) para carregar o v2"
echo "    2. rode paper-freeze num repo para conferir"
if compgen -G "$HOME/.*rc.paper-freeze-v2.bak" >/dev/null; then
  echo "    3. apague os *.paper-freeze-v2.bak depois de conferir (o do v1 tem o segredo em texto puro)"
  echo "    4. se veio do v1, rotacione a chave AWS: ela ficou em texto puro no rc"
fi
