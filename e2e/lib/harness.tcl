# harness.tcl is the shared expect library of adev's pty e2e suite. Every
# test sources it, calls boot, and then alternates await/send steps.
#
# The rules encoded here cost real debugging time; do not relax them:
#
#   - Event-driven only. Every send is preceded by an await of an on-screen
#     marker proving the previous state rendered. Keys sent before the app
#     enters raw mode sit in the canonical buffer and arrive late and
#     coalesced (esc + q becomes alt+q), so never sleep-then-send.
#   - Answer the startup terminal queries or the app blocks waiting for
#     them: OSC 11 (background color) and CSI 6n (cursor position) are
#     answered inside await, whenever they show up.
#   - The pty needs an explicit size right after spawn; a 0x0 terminal
#     renders nothing.
#   - HOME points at the per-test fixture home, because discovery always
#     prepends the user's config dirs to every scan.
#   - Markers must be plain ASCII text that lipgloss renders as one styled
#     segment: styling inserts escape sequences between segments, so a
#     marker spanning two segments never matches.
#
# Environment (exported by run.sh): ADEV_BIN, E2E_WORK, E2E_HOME, E2E_ROOT,
# E2E_BETA. Set E2E_VERBOSE=1 to watch the raw pty stream.

set timeout 20
match_max 200000

if {![info exists env(E2E_VERBOSE)]} {
    log_user 0
}

# step narrates the test's progress, since the pty stream itself is silenced.
proc step {what} {
    puts "  $what"
}

# fail prints the reason plus the tail of the pty stream and exits nonzero.
# run.sh keeps the fixture dir (and the ADEV_TUI_LOG key trace) on failure.
proc fail {why} {
    global env expect_out
    if {[info exists expect_out(buffer)]} {
        puts stderr "\n--- last pty output ---"
        puts stderr [string range $expect_out(buffer) end-600 end]
        puts stderr "-----------------------"
    }
    if {[info exists env(ADEV_TUI_LOG)]} {
        puts stderr "key trace: $env(ADEV_TUI_LOG)"
    }
    puts stderr "FAIL: $why"
    exit 1
}

# await blocks until marker (a literal string) appears in the pty stream,
# answering any terminal query that arrives while waiting. Everything before
# the match is consumed, so consecutive awaits must follow the on-screen
# (top-to-bottom, repaint) order of their markers.
proc await {marker} {
    # expect populates expect_out in the caller's scope; declare it global
    # so fail (another proc) can dump the last matched buffer.
    global expect_out
    expect {
        -re {\x1b\]11;\?(\x07|\x1b\\)} {
            send -- "\x1b]11;rgb:0000/0000/0000\x1b\\"
            exp_continue
        }
        -re {\x1b\[6n} {
            send -- "\x1b\[1;1R"
            exp_continue
        }
        -ex $marker {}
        timeout { fail "timed out waiting for \"$marker\"" }
        eof { fail "unexpected EOF waiting for \"$marker\"" }
    }
}

# boot launches adev's TUI inside a sized pty against the fixture home and
# waits for the dashboard (the intro animation auto-finishes). The initial
# scan root is E2E_ROOT, because the TUI scans the directory it starts in.
proc boot {} {
    # spawn_id must be declared global or spawn sets it proc-local, and
    # every later expect silently reads expect's own stdin instead.
    global env spawn_out spawn_id

    set env(HOME) $env(E2E_HOME)
    set env(TERM) xterm-256color
    set env(ADEV_TUI_LOG) [file join $env(E2E_WORK) tui.log]

    cd $env(E2E_ROOT)
    spawn $env(ADEV_BIN)

    # A fresh pty is 0x0 and a 0x0 terminal renders nothing; sizing it sends
    # SIGWINCH, which bubbletea turns into its first real layout.
    exec stty rows 35 columns 120 < $spawn_out(slave,name)

    # "Paths" is the left panel title of the dashboard's default view: its
    # appearance marks both raw mode and the end of the intro.
    await "Paths"
}

# quit exits the app from normal mode and waits for the process to end.
proc quit {} {
    send "q"
    expect {
        eof {}
        timeout { fail "app did not exit on q" }
    }
}
