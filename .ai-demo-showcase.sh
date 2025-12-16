#!/bin/bash

#==============================================================================
#  AI-ASSISTED CODING SHOWCASE - LOOM DEMO FOR OLIV AI
#==============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GRAY='\033[0;90m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

declare -a CONTENT

add_content() {
    CONTENT+=("$1")
}

build_content() {

#==============================================================================
# SECTION 1: INTRO
#==============================================================================

add_content ""
add_content "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}        ${WHITE}${BOLD}🚀 AI-ASSISTED CODING DEMO 🚀${NC}                                        ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}        ${GRAY}Sahil Chouksey | Full-Stack Engineer${NC}                                  ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}What I'll demonstrate:${NC}"
add_content ""
add_content "    ${GREEN}1.${NC} ${WHITE}AI-assisted coding as a ${BOLD}productivity multiplier${NC}"
add_content "    ${GREEN}2.${NC} ${WHITE}My ${BOLD}Research → Plan → Implement${NC} ${WHITE}workflow${NC}"
add_content "    ${GREEN}3.${NC} ${WHITE}Building a feature ${BOLD}live${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 2: MY APPROACH
#==============================================================================

add_content "${YELLOW}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${YELLOW}${BOLD}  💡 MY APPROACH: NOT VIBE CODING${NC}"
add_content "${YELLOW}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}LLMs are stateless. Quality output = Quality context.${NC}"
add_content ""
add_content "  ${CYAN}${BOLD}I optimize for:${NC}"
add_content ""
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Correctness${NC}   ${GRAY}─ Right information in context${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Completeness${NC}  ${GRAY}─ All relevant code loaded${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Minimal noise${NC} ${GRAY}─ No irrelevant distractions${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 3: THE WORKFLOW
#==============================================================================

add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${GREEN}${BOLD}  🔄 WORKFLOW: RESEARCH → PLAN → IMPLEMENT${NC}"
add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${BLUE}[RESEARCH]${NC} ${GRAY}═══▶${NC} ${GREEN}[PLAN]${NC} ${GRAY}═══▶${NC} ${YELLOW}[IMPLEMENT]${NC}"
add_content ""
add_content "       ${BLUE}│${NC}              ${GREEN}│${NC}              ${YELLOW}│${NC}"
add_content "       ${BLUE}▼${NC}              ${GREEN}▼${NC}              ${YELLOW}▼${NC}"
add_content "  ${DIM}Understand${NC}      ${DIM}Spec with${NC}      ${DIM}Execute &${NC}"
add_content "  ${DIM}codebase${NC}        ${DIM}exact steps${NC}    ${DIM}verify${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}Human review at each step.${NC}"
add_content ""
add_content "  ${RED}Why?${NC} Bad research = 1000s of bad lines downstream."
add_content ""
add_content ""

#==============================================================================
# SECTION 4: OWNERSHIP
#==============================================================================

add_content "${MAGENTA}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${MAGENTA}${BOLD}  🎯 OWNERSHIP & INITIATIVE${NC}"
add_content "${MAGENTA}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${GREEN}★${NC} ${WHITE}${BOLD}Own the outcome${NC}${WHITE}, not just the task${NC}"
add_content "  ${GREEN}★${NC} ${WHITE}${BOLD}Proactive${NC}${WHITE} - fix problems before asked${NC}"
add_content "  ${GREEN}★${NC} ${WHITE}${BOLD}Ship production-grade${NC}${WHITE} - tests, error handling, first time${NC}"
add_content ""
add_content ""
add_content "  ${CYAN}Example from BRIO Health AI:${NC}"
add_content ""
add_content "    ${GREEN}✓${NC} ${WHITE}Mem0.ai integration → ${GREEN}2X response quality${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}Custom OCR pipeline → ${GREEN}60% faster processing${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 5: STARTUP RIGOR
#==============================================================================

add_content "${RED}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${RED}${BOLD}  🔥 STARTUP RIGOR${NC}"
add_content "${RED}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${YELLOW}1.${NC} ${WHITE}${BOLD}Move fast with intention${NC} ${GRAY}─ plan before code${NC}"
add_content "  ${YELLOW}2.${NC} ${WHITE}${BOLD}First-principles${NC} ${GRAY}─ simplest solution that works${NC}"
add_content "  ${YELLOW}3.${NC} ${WHITE}${BOLD}Ship & iterate${NC} ${GRAY}─ get feedback early${NC}"
add_content "  ${YELLOW}4.${NC} ${WHITE}${BOLD}AI as multiplier${NC} ${GRAY}─ I review everything generated${NC}"
add_content ""
add_content ""
add_content "  ${GRAY}\"This isn't vibe coding. This is ${WHITE}engineering with intention${GRAY}.\"${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 6: LIVE DEMO
#==============================================================================

add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${GREEN}${BOLD}  🎬 LIVE DEMO${NC}"
add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}Task:${NC} ${CYAN}Add enhanced health check endpoint${NC}"
add_content ""
add_content "  ${WHITE}Returns: status, timestamp, uptime, database status${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}Watch for:${NC}"
add_content ""
add_content "    ${GREEN}1.${NC} ${WHITE}How I give context to AI${NC}"
add_content "    ${GREEN}2.${NC} ${WHITE}How I review generated code${NC}"
add_content "    ${GREEN}3.${NC} ${WHITE}The conversation flow${NC}"
add_content ""
add_content ""
add_content "  ${YELLOW}${BOLD}>>> Switching to opencode now <<<${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 7: CLOSING
#==============================================================================

add_content "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}        ${WHITE}${BOLD}Thanks for watching! 🚀${NC}                                               ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}        ${GRAY}Sahil Chouksey | hey@sahilchouksey.in${NC}                                 ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
add_content ""
add_content ""
add_content "  ${DIM}Press 'q' to exit${NC}"
add_content ""

}

#==============================================================================
# DISPLAY FUNCTION
#==============================================================================

get_term_size() {
    TERM_LINES=$(tput lines)
    TERM_COLS=$(tput cols)
}

cleanup() {
    tput cnorm
    clear
    echo -e "\n${GREEN}${BOLD}Thanks for watching! 🚀${NC}\n"
}

display_content() {
    local current_line=0
    local lines_per_page=5
    local total_lines=${#CONTENT[@]}
    
    trap cleanup EXIT
    
    clear
    tput civis
    get_term_size
    
    local content_start_row=4
    local content_end_row=$((TERM_LINES - 3))
    local max_visible_lines=$((content_end_row - content_start_row))
    local window_start=0
    
    while true; do
        clear
        
        # Header
        tput cup 0 0
        echo -e "${GRAY}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${WHITE}  ${CYAN}${BOLD}AI CODING DEMO${NC} ${GRAY}│${NC} ${GREEN}ENTER${NC}=next ${YELLOW}b${NC}=back ${RED}q${NC}=quit"
        echo -e "${GRAY}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        
        # Adjust window
        if [ $current_line -gt $((window_start + max_visible_lines)) ]; then
            window_start=$((current_line - max_visible_lines + lines_per_page))
        fi
        if [ $window_start -lt 0 ]; then
            window_start=0
        fi
        
        # Content
        tput cup $content_start_row 0
        local lines_drawn=0
        for ((i=window_start; i<current_line && lines_drawn<max_visible_lines; i++)); do
            echo -e "${CONTENT[$i]}"
            ((lines_drawn++))
        done
        
        # Progress bar
        tput cup $((TERM_LINES - 2)) 0
        
        local progress=$((current_line * 100 / total_lines))
        local bar_width=40
        local filled=$((progress * bar_width / 100))
        local empty=$((bar_width - filled))
        
        local bar_filled=""
        local bar_empty=""
        for ((j=0; j<filled; j++)); do bar_filled+="█"; done
        for ((j=0; j<empty; j++)); do bar_empty+="░"; done
        
        echo -e "${GRAY}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "  ${WHITE}Progress: ${GREEN}[${bar_filled}${GRAY}${bar_empty}${GREEN}]${NC} ${WHITE}${progress}%${NC}"
        
        if [ $current_line -ge $total_lines ]; then
            tput cup $((content_start_row + lines_drawn + 1)) 0
            echo -e "  ${GREEN}${BOLD}✓ End.${NC} ${WHITE}Press 'r' to restart or 'q' to quit.${NC}"
        fi
        
        read -rsn1 input
        
        case "$input" in
            q|Q) exit 0 ;;
            b|B)
                current_line=$((current_line - 10))
                if [ $current_line -lt 0 ]; then current_line=0; fi
                window_start=$((current_line - max_visible_lines + 5))
                if [ $window_start -lt 0 ]; then window_start=0; fi
                ;;
            r|R)
                current_line=0
                window_start=0
                ;;
            *)
                if [ $current_line -lt $total_lines ]; then
                    current_line=$((current_line + lines_per_page))
                    if [ $current_line -gt $total_lines ]; then
                        current_line=$total_lines
                    fi
                fi
                ;;
        esac
    done
}

#==============================================================================
# MAIN
#==============================================================================

build_content
display_content
