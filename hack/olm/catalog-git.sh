#!/usr/bin/env bash

prepare_catalog_checkout() {
  if [[ $# -ne 4 ]]; then
    echo "usage: prepare_catalog_checkout <fork> <upstream-repository> <branch> <checkout-path>" >&2
    return 2
  fi

  local fork=$1
  local upstream_repository=$2
  local branch=$3
  local checkout_path=$4

  git config --global user.name "github-actions[bot]"
  git config --global user.email "41898282+github-actions[bot]@users.noreply.github.com"
  gh auth setup-git

  gh repo clone "$fork" "$checkout_path" >/dev/null
  if git -C "$checkout_path" remote get-url upstream >/dev/null 2>&1; then
    git -C "$checkout_path" remote set-url upstream "https://github.com/$upstream_repository.git"
  else
    git -C "$checkout_path" remote add upstream "https://github.com/$upstream_repository.git"
  fi
  git -C "$checkout_path" remote set-url origin "https://github.com/$fork.git"
  git -C "$checkout_path" fetch --quiet upstream main
  git -C "$checkout_path" checkout --quiet -B "$branch" upstream/main
}
