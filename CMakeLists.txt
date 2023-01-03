cmake_minimum_required(VERSION 3.16)
project(tanabata
        VERSION 1.0.0
        HOMEPAGE_URL https://github.com/H1K0/tanabata
        LANGUAGES C)

set(CMAKE_C_STANDARD 99)

set(CORE_SRC
        include/core.h
        core/sasahyou.c
        core/sappyou.c
        core/shoppyou.c)

set(TANABATA_SRC
        ${CORE_SRC}
        include/tanabata.h
        lib/database.c
        lib/sasa.c
        lib/tanzaku.c
        lib/kazari.c)

set(CLI_SRC
        cli/cli.c)

# Tanabata shared lib
add_library(tanabata SHARED ${TANABATA_SRC})

# Tanabata CLI app
add_executable(tfm ${TANABATA_SRC} ${CLI_SRC})

# Authentication CGI app
add_executable(tfm-cgi ${TANABATA_SRC} cgi/cgi.c)
target_link_libraries(tfm-cgi fcgi ssl crypto pthread)
