# Tcl on Windows has unfortunate defaults:
#   - cp1252 encoding, which will mangle utf-8 source code
#   - crlf linebreaks instead of unix-style lf
# Let's be consistent cross-platform to avoid surprises:
encoding system "utf-8"
foreach p {stdin stdout stderr} {
    fconfigure $p -encoding "utf-8"
    fconfigure $p -translation lf
}

package require Tk

wm title . "WebMaker2000"
tk appname webmaker2000

set OS [lindex $tcl_platform(os) 0]
if {$OS == "Windows"} {
    ttk::style theme use vista
} elseif {$OS == "Darwin"} {
    ttk::style theme use aqua
} else {
    ttk::style theme use clam
}

wm protocol . WM_DELETE_WINDOW {
    exit 0
}

proc initialize {} {
    # By default this window is not focused and not even brought to
    # foreground on Windows. I suspect it's because tcl is exec'ed from Go.
    # The old "iconify, deiconify" trick no longer seems to work, so this time
    # I'm passing it to Go to call the winapi's SetForegroundWindow directly.
    if {$::tcl_platform(platform) == "windows"} {
        windows_forcefocus
    }
}

# Very simple line-based IPC system where Tcl client talks to Go server
# via stdin/stdout
proc ipc_write {method args} {
    puts "$method [llength $args]"
    foreach a $args {
        puts "$a"
    }
}
proc ipc_read {} {
    set results {}
    set numlines [gets stdin]
    for {set i 0} {$i < $numlines} {incr i} {
        lappend results [gets stdin]
    }
    return $results
}
proc ipc {method args} {
    ipc_write $method {*}$args
    return [ipc_read]
}

proc windows_forcefocus {} {
    # First call winapi's SetForegroundWindow()
    set handle [winfo id .]
    ipc "forcefocus" $handle
    # Then call force focus on tcl side
    focus -force .
    # We must do both in order to properly focus on main tk window.
    # Don't ask me why - that's just how it works.
    #
    # Alternatively we can try making Tcl our entrypoint instead of exec-ing
    # Tcl from Go. Maybe some other time.
}

proc loadicon {} {
    set iconblob [image create photo -file gorts.png]
    wm iconphoto . -default $iconblob
}

proc loadstartgg {} {
    set resp [ipc "getstartgg"]
    set ::startgg(token) [lindex $resp 0]
    set ::startgg(slug) [lindex $resp 1]
}

proc loadwebmsg {} {
    set resp [ipc "getwebport"]
    set webport [lindex $resp 0]
    set ::mainstatus "Point your OBS browser source to http://localhost:${webport}"
}

proc loadcountrycodes {} {
    set codes [ipc "getcountrycodes"]
    .n.m.players.p1country configure -values $codes
    .n.m.players.p2country configure -values $codes
}

proc loadscoreboard {} {
    set sb [ipc "getscoreboard"]
    set ::scoreboard(description) [lindex $sb 0]
    set ::scoreboard(subtitle) [lindex $sb 1]
    set ::scoreboard(p1name) [lindex $sb 2]
    set ::scoreboard(p1country) [lindex $sb 3]
    set ::scoreboard(p1score) [lindex $sb 4]
    set ::scoreboard(p1team) [lindex $sb 5]
    set ::scoreboard(p2name) [lindex $sb 6]
    set ::scoreboard(p2country) [lindex $sb 7]
    set ::scoreboard(p2score) [lindex $sb 8]
    set ::scoreboard(p2team) [lindex $sb 9]
    update_applied_scoreboard
}

proc applyscoreboard {} {
    set sb [ \
        ipc "applyscoreboard" \
        $::scoreboard(description) \
        $::scoreboard(subtitle) \
        $::scoreboard(p1name) \
        $::scoreboard(p1country) \
        $::scoreboard(p1score) \
        $::scoreboard(p1team) \
        $::scoreboard(p2name) \
        $::scoreboard(p2country) \
        $::scoreboard(p2score) \
        $::scoreboard(p2team) \
    ]
    update_applied_scoreboard
}

proc loadplayernames {} {
    set playernames [ipc "searchplayers" ""]
    .n.m.players.p1name configure -values $playernames
    .n.m.players.p2name configure -values $playernames
}

proc setupplayersuggestion {} {
    proc update_suggestions {_ key _} {
        if {!($key == "p1name" || $key == "p2name")} {
            return
        }
        set newvalue $::scoreboard($key)
        set widget .n.m.players.$key
        set matches [ipc "searchplayers" $newvalue]
        $widget configure -values $matches

        if {[llength matches] == 1 && [lindex $matches 0] == $newvalue} {
            set countryvar "p[string index $key 1]country"
            set country [lindex [ipc "getplayercountry" $newvalue] 0]
            set ::scoreboard($countryvar) $country
        }
    }
    trace add variable ::scoreboard write update_suggestions
}

proc fetchplayers {} {
    if {$::startgg(token) == "" || $::startgg(slug) == ""} {
        set ::startgg(msg) "Please enter token & slug first."
        return
    }
    .n.s.buttons.fetch configure -state disabled
    .n.s.buttons.clear configure -state disabled
    .n.s.token configure -state disabled
    .n.s.tournamentslug configure -state disabled
    .n state disabled
    set ::startgg(msg) "Fetching..."
    ipc_write "fetchplayers" $::startgg(token) $::startgg(slug)
}

proc fetchplayers__resp {} {
    set resp [ipc_read]
    set status [lindex $resp 0]
    set msg [lindex $resp 1]

    set ::startgg(msg) $msg

    if {$status == "ok"} {
        loadplayernames
    }

    .n.s.buttons.fetch configure -state normal
    .n.s.buttons.clear configure -state normal
    .n.s.token configure -state normal
    .n.s.tournamentslug configure -state normal
    .n state !disabled
}

proc clearstartgg {} {
    set ::startgg(token) ""
    set ::startgg(slug) ""
    set ::startgg(msg) ""
    ipc_write "clearstartgg"
}

proc discardscoreboard {} {
    foreach key [array names ::scoreboard] {
        set ::scoreboard($key) $::applied_scoreboard($key)
    }
    # Country is updated whenever player name is updated,
    # so make sure we set countries last.
    set ::scoreboard(p1country) $::applied_scoreboard(p1country)
    set ::scoreboard(p2country) $::applied_scoreboard(p2country)
}

proc update_applied_scoreboard {} {
    foreach key [array names ::scoreboard] {
        set ::applied_scoreboard($key) $::scoreboard($key)
    }
}

proc setupdiffcheck {} {
    # Define styling for "dirty"
    foreach x {TEntry TCombobox TSpinbox} {
        ttk::style configure "Dirty.$x" -fieldbackground #dffcde
    }

    trace add variable ::scoreboard write ::checkdiff
    trace add variable ::applied_scoreboard write ::checkdiff
}

proc checkdiff {_ key _} {
    set widget $::var_to_widget($key)
    if {$::scoreboard($key) == $::applied_scoreboard($key)} {
        $widget configure -style [winfo class $widget]
    } else {
        $widget configure -style "Dirty.[winfo class $widget]"
    }
}
