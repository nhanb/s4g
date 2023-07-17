package require Tk

wm title . "Create or Open?"

set OS [lindex $tcl_platform(os) 0]
if {$OS == "Windows"} {
    ttk::style theme use vista
} elseif {$OS == "Darwin"} {
    ttk::style theme use aqua
} else {
    ttk::style theme use clam
}

set types {
    {{WebMaker2000 Files} {.wbmkr2k}}
}

ttk::frame .c -padding "10"
ttk::label .c.label -text {Would you like to create a new blog, or open an existing one?}
ttk::button .c.createBtn -text "Create..." -padding 5 -command {
    #set filename [tk_getSaveFile -title "Create" -filetypes $types]
    set filename [tk_chooseDirectory]
    if {$filename != ""} {
        puts "create ${filename}"
        exit
    }
}
ttk::button .c.openBtn -text "Open..." -padding 5 -command {
    set filename [tk_getOpenFile -filetypes $types]
    if {$filename != ""} {
        puts "open ${filename}"
        exit
    }
}

grid .c -column 0 -row 0
grid .c.label -column 0 -row 0 -columnspan 2 -pady "0 10"
grid .c.createBtn -column 0 -row 1 -padx 10
grid .c.openBtn -column 1 -row 1 -padx 10

tk::PlaceWindow . center
