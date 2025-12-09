#!/bin/bash

#==============================================================================
#  AI-ASSISTED CODING SHOWCASE - LOOM DEMO FOR OLIV AI
#  
#  Demonstrating:
#  - Expertise with AI-assisted coding (Context Engineering)
#  - Ownership & Initiative
#  - Rigor for startup
#
#  Usage: ./ai-demo-showcase.sh
#  Navigation: ENTER=next, b=back, q=quit, r=restart
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
ITALIC='\033[3m'
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
add_content "${CYAN}${BOLD}║${NC}     ${WHITE}${BOLD}🚀 AI-ASSISTED CODING DEMO FOR OLIV AI 🚀${NC}                               ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}     ${GRAY}Sahil Chouksey | Full-Stack Engineer${NC}                                    ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}What I'll demonstrate:${NC}"
add_content ""
add_content "    ${GREEN}1.${NC} ${WHITE}How I use AI-assisted coding as a ${BOLD}productivity multiplier${NC}"
add_content "    ${GREEN}2.${NC} ${WHITE}My ${BOLD}Research → Plan → Implement${NC} ${WHITE}workflow${NC}"
add_content "    ${GREEN}3.${NC} ${WHITE}Building a real feature ${BOLD}live${NC} ${WHITE}with context engineering${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 2: MY APPROACH - NOT VIBE CODING
#==============================================================================

add_content "${YELLOW}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${YELLOW}${BOLD}  💡 MY APPROACH: CONTEXT ENGINEERING${NC}"
add_content "${YELLOW}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${RED}${BOLD}This is NOT \"vibe coding\"${NC}"
add_content ""
add_content "  ${WHITE}LLMs are stateless functions:${NC}"
add_content "    ${GRAY}→${NC} ${WHITE}Quality output ${GREEN}=${NC} ${WHITE}Quality input (context)${NC}"
add_content "    ${GRAY}→${NC} ${WHITE}I ${BOLD}engineer${NC} ${WHITE}the context, not just prompt${NC}"
add_content ""
add_content ""
add_content "  ${CYAN}${BOLD}What I optimize for:${NC}"
add_content ""
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Correctness${NC}   ${GRAY}─ No incorrect information in context${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Completeness${NC}  ${GRAY}─ All relevant code/docs loaded${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Size${NC}          ${GRAY}─ Minimal noise, maximum signal${NC}"
add_content "    ${GREEN}✓${NC} ${WHITE}${BOLD}Trajectory${NC}    ${GRAY}─ Right direction for the task${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 3: THE WORKFLOW
#==============================================================================

add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${GREEN}${BOLD}  🔄 MY WORKFLOW: RESEARCH → PLAN → IMPLEMENT${NC}"
add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${BLUE}╔═════════════════╗${NC}       ${GREEN}╔═════════════════╗${NC}       ${YELLOW}╔═════════════════╗${NC}"
add_content "  ${BLUE}║${NC}                 ${BLUE}║${NC}       ${GREEN}║${NC}                 ${GREEN}║${NC}       ${YELLOW}║${NC}                 ${YELLOW}║${NC}"
add_content "  ${BLUE}║${NC}    ${WHITE}${BOLD}RESEARCH${NC}     ${BLUE}║${NC} ${GRAY}═══▶${NC}  ${GREEN}║${NC}      ${WHITE}${BOLD}PLAN${NC}       ${GREEN}║${NC} ${GRAY}═══▶${NC}  ${YELLOW}║${NC}   ${WHITE}${BOLD}IMPLEMENT${NC}    ${YELLOW}║${NC}"
add_content "  ${BLUE}║${NC}                 ${BLUE}║${NC}       ${GREEN}║${NC}                 ${GREEN}║${NC}       ${YELLOW}║${NC}                 ${YELLOW}║${NC}"
add_content "  ${BLUE}╚═════════════════╝${NC}       ${GREEN}╚═════════════════╝${NC}       ${YELLOW}╚═════════════════╝${NC}"
add_content "          ${BLUE}│${NC}                       ${GREEN}│${NC}                       ${YELLOW}│${NC}"
add_content "          ${BLUE}▼${NC}                       ${GREEN}▼${NC}                       ${YELLOW}▼${NC}"
add_content "  ${BLUE}┌─────────────────┐${NC}       ${GREEN}┌─────────────────┐${NC}       ${YELLOW}┌─────────────────┐${NC}"
add_content "  ${BLUE}│${NC} ${DIM}Explore codebase${NC} ${BLUE}│${NC}       ${GREEN}│${NC} ${DIM}Write spec with${NC}  ${GREEN}│${NC}       ${YELLOW}│${NC} ${DIM}Execute phase${NC}    ${YELLOW}│${NC}"
add_content "  ${BLUE}│${NC} ${DIM}Find patterns${NC}    ${BLUE}│${NC}       ${GREEN}│${NC} ${DIM}exact changes${NC}    ${GREEN}│${NC}       ${YELLOW}│${NC} ${DIM}by phase with${NC}    ${YELLOW}│${NC}"
add_content "  ${BLUE}│${NC} ${DIM}Understand flow${NC}  ${BLUE}│${NC}       ${GREEN}│${NC} ${DIM}& test criteria${NC}  ${GREEN}│${NC}       ${YELLOW}│${NC} ${DIM}verification${NC}     ${YELLOW}│${NC}"
add_content "  ${BLUE}└─────────────────┘${NC}       ${GREEN}└─────────────────┘${NC}       ${YELLOW}└─────────────────┘${NC}"
add_content ""
add_content ""
add_content "        ${WHITE}${BOLD}▲${NC}                                                     ${WHITE}${BOLD}│${NC}"
add_content "        ${WHITE}${BOLD}│${NC}               ${MAGENTA}${BOLD}HUMAN REVIEW AT EACH STEP${NC}              ${WHITE}${BOLD}│${NC}"
add_content "        ${WHITE}${BOLD}└─────────────────────────────────────────────────────┘${NC}"
add_content ""
add_content ""
add_content "  ${RED}${BOLD}Why this matters:${NC}"
add_content ""
add_content "    ${GRAY}•${NC} ${WHITE}Bad code = 1 bad line${NC}"
add_content "    ${GRAY}•${NC} ${YELLOW}Bad plan = ${RED}100s${NC} ${YELLOW}of bad lines${NC}"
add_content "    ${GRAY}•${NC} ${RED}Bad research = ${WHITE}${BOLD}1000s${NC} ${RED}of bad lines${NC}"
add_content ""
add_content "  ${CYAN}I focus my attention on the ${BOLD}highest leverage${NC} ${CYAN}points.${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 4: OWNERSHIP & INITIATIVE
#==============================================================================

add_content "${MAGENTA}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${MAGENTA}${BOLD}  🎯 OWNERSHIP & INITIATIVE${NC}"
add_content "${MAGENTA}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}How I approach work:${NC}"
add_content ""
add_content "    ${GREEN}★${NC} ${WHITE}${BOLD}Own the outcome${NC}${WHITE}, not just the task${NC}"
add_content "       ${GRAY}→ If something's broken, I fix it. Don't wait to be asked.${NC}"
add_content ""
add_content "    ${GREEN}★${NC} ${WHITE}${BOLD}Proactive problem-solving${NC}"
add_content "       ${GRAY}→ Identify issues before they become blockers.${NC}"
add_content ""
add_content "    ${GREEN}★${NC} ${WHITE}${BOLD}Ship production-grade code${NC}"
add_content "       ${GRAY}→ Tests, error handling, edge cases - the first time.${NC}"
add_content ""
add_content "    ${GREEN}★${NC} ${WHITE}${BOLD}Transparent communication${NC}"
add_content "       ${GRAY}→ Share progress, blockers, and learnings openly.${NC}"
add_content ""
add_content ""
add_content "  ${CYAN}${BOLD}Real example from my work at BRIO Health AI:${NC}"
add_content ""
add_content "    ${WHITE}Task: Build RAG system for medical research${NC}"
add_content ""
add_content "    ${GRAY}What I did beyond the ticket:${NC}"
add_content "      ${GREEN}✓${NC} ${WHITE}Implemented Mem0.ai for context memory → ${GREEN}2X response quality${NC}"
add_content "      ${GREEN}✓${NC} ${WHITE}Built custom OCR pipeline → ${GREEN}60% faster PDF processing${NC}"
add_content "      ${GREEN}✓${NC} ${WHITE}Designed modular architecture → ${GREEN}40% faster iteration${NC}"
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
add_content "  ${WHITE}${BOLD}My engineering principles:${NC}"
add_content ""
add_content "    ${YELLOW}1.${NC} ${WHITE}${BOLD}Move fast with intention${NC}"
add_content "       ${GRAY}Speed matters, but so does direction. I plan before I code.${NC}"
add_content ""
add_content "    ${YELLOW}2.${NC} ${WHITE}${BOLD}First-principles thinking${NC}"
add_content "       ${GRAY}Question assumptions. Find the simplest solution that works.${NC}"
add_content ""
add_content "    ${YELLOW}3.${NC} ${WHITE}${BOLD}Ship & iterate${NC}"
add_content "       ${GRAY}Perfect is the enemy of shipped. Get feedback early.${NC}"
add_content ""
add_content "    ${YELLOW}4.${NC} ${WHITE}${BOLD}AI as a multiplier, not a crutch${NC}"
add_content "       ${GRAY}I understand what the AI generates. I review, I verify.${NC}"
add_content ""
add_content ""
add_content "  ${GRAY}╔════════════════════════════════════════════════════════════════════╗${NC}"
add_content "  ${GRAY}║${NC}                                                                    ${GRAY}║${NC}"
add_content "  ${GRAY}║${NC}   ${WHITE}\"This isn't vibe coding. This is ${BOLD}engineering with intention${NC}${WHITE}.\"${NC}  ${GRAY}║${NC}"
add_content "  ${GRAY}║${NC}                                                                    ${GRAY}║${NC}"
add_content "  ${GRAY}╚════════════════════════════════════════════════════════════════════╝${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 6: LIVE DEMO INTRO
#==============================================================================

add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content "${GREEN}${BOLD}  🎬 LIVE DEMO: BUILDING A FEATURE WITH AI${NC}"
add_content "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}What I'll build now:${NC}"
add_content ""
add_content "    ${CYAN}Adding a new API endpoint with full CRUD operations${NC}"
add_content "    ${GRAY}(Using my existing study-in-woods project)${NC}"
add_content ""
add_content ""
add_content "  ${WHITE}${BOLD}Watch for:${NC}"
add_content ""
add_content "    ${GREEN}1.${NC} ${WHITE}How I provide context to the AI${NC}"
add_content "    ${GREEN}2.${NC} ${WHITE}How I review and verify generated code${NC}"
add_content "    ${GREEN}3.${NC} ${WHITE}How I iterate based on results${NC}"
add_content "    ${GREEN}4.${NC} ${WHITE}The ${BOLD}conversation${NC} ${WHITE}between human and AI${NC}"
add_content ""
add_content ""
add_content "  ${YELLOW}${BOLD}>>> Switching to opencode terminal now <<<${NC}"
add_content ""
add_content ""

#==============================================================================
# SECTION 7: CLOSING
#==============================================================================

add_content "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}     ${WHITE}${BOLD}Thanks for watching! 🚀${NC}                                                 ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}                                                                              ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}     ${GRAY}Sahil Chouksey${NC}                                                           ${CYAN}${BOLD}║${NC}"
add_content "${CYAN}${BOLD}║${NC}     ${GRAY}hey@sahilchouksey.in | sahilchouksey.in${NC}                                  ${CYAN}${BOLD}║${NC}"
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
        echo -e "${WHITE}  ${CYAN}${BOLD}OLIV AI DEMO${NC} ${GRAY}│${NC} ${GREEN}ENTER${NC}${WHITE}=next ${YELLOW}b${NC}${WHITE}=back ${RED}q${NC}${WHITE}=quit ${CYAN}r${NC}${WHITE}=restart${NC}"
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
        echo -e "  ${WHITE}Progress: ${GREEN}[${bar_filled}${GRAY}${bar_empty}${GREEN}]${NC} ${WHITE}${progress}%${NC} ${GRAY}(${current_line}/${total_lines})${NC}"
        
        if [ $current_line -ge $total_lines ]; then
            tput cup $((content_start_row + lines_drawn + 1)) 0
            echo -e "  ${GREEN}${BOLD}✓ End.${NC} ${WHITE}Press ${CYAN}'r'${WHITE} to restart or ${RED}'q'${WHITE} to quit.${NC}"
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

main() {
    case "${1:-}" in
        --help|-h)
            echo ""
            echo -e "${CYAN}${BOLD}Oliv AI Demo Script${NC}"
            echo ""
            echo "Usage: $0"
            echo ""
            echo "Controls: ENTER=next, b=back, r=restart, q=quit"
            echo ""
            ;;
        *)
            build_content
            display_content
            ;;
    esac
}

main "$@"
