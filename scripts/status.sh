#!/usr/bin/env bash
# Muxio global status. Derived from repository state on every run, never stored,
# so it cannot drift out of date. Task progress still lives in GitHub Issues.
set -euo pipefail
cd "$(dirname "$0")/.."

rule() { printf '%s\n' "-------------------------------------------------------------"; }

# field extracts a "- key：value" line from a document front block.
field() { grep -m1 "^- $2：" "$1" | sed "s/^- $2：//" | tr -d '\r'; }

# title extracts the document H1 without its numeric prefix.
title() { grep -m1 '^# ' "$1" | sed 's/^# //;s/^Spec [0-9]*：//;s/^ADR-[0-9]*：//'; }

printf 'Muxio 全局状态  (%s)\n' "$(date '+%Y-%m-%d %H:%M')"
rule

printf '\n里程碑范围 (docs/roadmap.md)\n'
grep -E '^\| M[0-9]' docs/roadmap.md | while IFS='|' read -r _ milestone content _; do
  printf '  %s — %s\n' "$(echo "$milestone" | xargs)" "$(echo "$content" | xargs)"
done

printf '\nM1 切片顺序\n'
grep -E '^\| [0-9] \|' docs/roadmap.md | while IFS='|' read -r _ order slice carrier assumption _; do
  printf '  %s. %s  [%s]\n' "$(echo "$order" | xargs)" "$(echo "$slice" | xargs)" \
    "$(echo "$carrier" | sed 's/\[\([^]]*\)\](.*/\1/' | xargs)"
  printf '     验证：%s\n' "$(echo "$assumption" | xargs)"
done

printf '\nSpec\n'
for spec in docs/specs/[0-9][0-9][0-9]-*.md; do
  printf '  %s  %s  —  %s\n' "$(basename "$spec" | cut -d- -f1)" \
    "$(field "$spec" 状态)" "$(title "$spec")"
done

printf '\nADR\n'
for adr in docs/decisions/[0-9][0-9][0-9]-*.md; do
  printf '  %s  %s  —  %s\n' "$(basename "$adr" | cut -d- -f1)" \
    "$(field "$adr" 状态)" "$(title "$adr")"
done

# direct_dependencies counts require entries that are not marked indirect,
# across both the single-line and block forms of the directive.
direct_dependencies() {
  awk '
    /^require \(/ { block = 1; next }
    /^\)/         { block = 0; next }
    /\/\/ indirect/ { next }
    (block || /^require /) && /[[:space:]]v[0-9]/ { count++ }
    END { print count + 0 }
  ' go.mod
}

printf '\n代码\n'
printf '  Go 源文件 %s，测试文件 %s，直接依赖 %s\n' \
  "$(find cmd internal -name '*.go' -not -name '*_test.go' | wc -l | xargs)" \
  "$(find cmd internal -name '*_test.go' | wc -l | xargs)" \
  "$(direct_dependencies)"
printf '  已发布端点 %s\n' "$(grep -cE '^  /' api/openapi.yaml | xargs)"

printf '\nGit\n'
printf '  分支 %s，未提交改动 %s 个文件\n' \
  "$(git branch --show-current)" "$(git status --porcelain | wc -l | xargs)"
printf '  最近 %s\n' "$(git log -1 --format='%h %s (%cr)')"

printf '\n需要你决定的事\n'
pending=0
for spec in docs/specs/[0-9][0-9][0-9]-*.md; do
  number=$(basename "$spec" | cut -d- -f1)
  status=$(field "$spec" 状态)
  [ "$status" = "已实现" ] && continue

  if [ "$status" = "草稿" ]; then
    printf '  · Spec %s「%s」仍是草稿，确认后才能开工\n' "$number" "$(title "$spec")"
    pending=1
  fi
  # An unimplemented spec proposing a dependency is still an open gate.
  if grep -q '^## 依赖' "$spec"; then
    printf '  · Spec %s 提出新增外部依赖，属于决策卡点（AGENTS.md）\n' "$number"
    pending=1
  fi
done
[ "$pending" -eq 0 ] && printf '  （无）\n'

rule
printf '决策卡点清单见 AGENTS.md；任务与进度见 GitHub Issues。\n'
