#!/bin/sh
# fixture.sh builds the throwaway tree one e2e test runs against, under the
# work dir passed as $1:
#
#   home/.claude          the fake user config: 2 skills, 1 folder plugin,
#                         1 folder marketplace (no Claude registry files, so
#                         discovery uses the folder fallback deterministically)
#   alpha-root/.claude    the initial scan root: 31 skills, so the skills
#                         view (2 + 31 = 33 groups) overflows a 35-row
#                         terminal (28 visible rows) and scrolling is real
#   beta-root/.claude     the rescan target of the root-input flow: 1 skill
#
# The tests assert on these exact counts; change them together.
set -eu

WORK="$1"

# skill <skills-dir> <name> writes a minimal skill with parseable frontmatter.
skill() {
    mkdir -p "$1/$2"
    cat > "$1/$2/SKILL.md" <<EOF
---
name: $2
description: "Fixture skill $2"
---

# $2
EOF
}

HOME_CFG="$WORK/home/.claude"
skill "$HOME_CFG/skills" doomed-skill
skill "$HOME_CFG/skills" home-skill

mkdir -p "$HOME_CFG/plugins/demo-plugin/.claude-plugin"
cat > "$HOME_CFG/plugins/demo-plugin/.claude-plugin/plugin.json" <<EOF
{
  "name": "demo-plugin",
  "version": "1.2.3",
  "description": "Fixture plugin"
}
EOF

mkdir -p "$HOME_CFG/marketplaces/demo-market/.claude-plugin"
cat > "$HOME_CFG/marketplaces/demo-market/.claude-plugin/marketplace.json" <<EOF
{
  "name": "demo-market",
  "plugins": [
    { "name": "cat-plugin-one" },
    { "name": "cat-plugin-two" }
  ]
}
EOF

ALPHA_CFG="$WORK/alpha-root/.claude"
skill "$ALPHA_CFG/skills" proj-skill
for i in $(seq -w 1 30); do
    skill "$ALPHA_CFG/skills" "scroll-$i"
done

BETA_CFG="$WORK/beta-root/.claude"
skill "$BETA_CFG/skills" beta-skill
