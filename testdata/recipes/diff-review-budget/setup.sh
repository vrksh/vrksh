#!/usr/bin/env bash
# Makes a git repo on branch main with one uncommitted change, so `git diff main` has something to show.
set -e
git init -q -b main .
git config user.name recipe
git config user.email recipe@example.com
printf 'one\ntwo\nthree\n' > notes.txt
git add notes.txt
git commit -q -m init
printf 'one\ntwo changed\nthree\nfour\n' > notes.txt
