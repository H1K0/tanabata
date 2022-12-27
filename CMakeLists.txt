cmake_minimum_required(VERSION 3.22)
project(tanabata
        VERSION 0.1.2
        HOMEPAGE_URL https://github.com/H1K0/tanabata
        LANGUAGES C)

set(CMAKE_C_STANDARD 99)

set(CORE_SRC
        src/core/sasahyou.c
        src/core/sappyou.c
        src/core/shoppyou.c)

set(TANABATA_SRC
        src/tanabata/database.c
        src/tanabata/sasa.c
        src/tanabata/tanzaku.c
        src/tanabata/kazari.c)

# Tanabata shared main lib
add_library(tanabata SHARED ${CORE_SRC} ${TANABATA_SRC})

# Tanabata CLI app
add_executable(tfm main.c include/core.h ${CORE_SRC} include/tanabata.h ${TANABATA_SRC} include/cli.h src/cli.c)
