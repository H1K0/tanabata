cmake_minimum_required(VERSION 3.16)

project(Tanabata
        VERSION 1.0.0
        HOMEPAGE_URL https://github.com/H1K0/tanabata
        LANGUAGES C
        )

set(CMAKE_C_STANDARD 99)

set(CORE_SRC
        include/core.h
        tanabata/core/core_func.h
        tanabata/core/sasahyou.c
        tanabata/core/sappyou.c
        tanabata/core/shoppyou.c
        )

set(TANABATA_SRC
        ${CORE_SRC}
        include/tanabata.h
        tanabata/lib/database.c
        tanabata/lib/sasa.c
        tanabata/lib/tanzaku.c
        tanabata/lib/kazari.c
        )

set(TDBMS_SERVER_SRC
        ${TANABATA_SRC}
        include/tdbms.h
        tdbms/server/tdbms-server.c
        )

set(TDBMS_CLIENT_SRC
        include/tdbms.h
        include/tdbms-client.h
        tdbms/client/tdbms-client.c
        )

set(CLI_SRC
        ${TANABATA_SRC}
        tfm/cli/tfm-cli.c
        )

# Tanabata shared lib
add_library(tanabata SHARED ${TANABATA_SRC})

# Tanabata DBMS server
add_executable(tdbms ${TDBMS_SERVER_SRC})

# Tanabata DMBS CLI client app
add_executable(tdb tdbms/cli/tdbms-cli.c ${TDBMS_CLIENT_SRC})

# Tanabata CLI app
add_executable(tfm ${CLI_SRC})
