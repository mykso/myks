# README demo. Re-record from this directory with: vhs demo.tape
# Needs myks, helm and git on PATH. The hidden setup scaffolds a project in a temp dir.
Output demo.gif
Require myks
Require git
Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 760
Set Padding 24
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

Hide
Type `export PS1='$ ' GIT_PAGER=cat && cd "$(mktemp -d)" && git init -q -b main && git commit -q --allow-empty -m init`
Enter
Type "git remote add origin https://github.com/example/my-gitops.git && myks init 2>/dev/null && grep -v '^min-version' .myks.yaml > c && mv c .myks.yaml"
Enter
Type "myks render >/dev/null 2>&1 && git add -A && git commit -qm render && mkdir -p envs/prod/_apps/httpbingo/helm && clear"
Enter
Sleep 20s
Show

Type "# Two clusters, dev and prod. Production needs three replicas:"
Sleep 600ms
Enter
Type "echo 'replicaCount: 3' > envs/prod/_apps/httpbingo/helm/httpbingo.yaml"
Sleep 400ms
Enter
Sleep 500ms
Type "myks render --smart-mode.only-print"
Enter
Sleep 2s
Type "myks render"
Enter
Sleep 3s
Type "git diff --stat"
Enter
Sleep 1500ms
Type "git diff -U1 rendered/"
Enter
Sleep 5s
