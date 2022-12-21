// Tanabata file manager lib
// By Masahiko AMANO aka H1K0

#ifndef TANABATA_H
#define TANABATA_H

#ifdef __cplusplus
#include <cstdint>
#include <cstdio>
extern "C" {
#else
#include <stdint.h>
#include <stdio.h>
#endif

#include "core.h"

// Tanabata (七夕) - the struct with all databases
typedef struct tanabata {
    Sasahyou  sasahyou;   // Sasahyou struct
    Sappyou   sappyou;    // Sappyou struct
    Shoppyou  shoppyou;   // Shoppyou struct
} Tanabata;

// ==================== DATABASE SECTION ==================== //

// Initialize empty tanabata
int tanabata_init(Tanabata *tanabata);

// Free tanabata
int tanabata_free(Tanabata *tanabata);

// Weed tanabata
int tanabata_weed(Tanabata *tanabata);

// Load tanabata
int tanabata_load(Tanabata *tanabata);

// Save tanabata
int tanabata_save(Tanabata *tanabata);

// Open tanabata
int tanabata_open(Tanabata *tanabata, const char *path);

// Dump tanabata
int tanabata_dump(Tanabata *tanabata, const char *path);

#ifdef __cplusplus
}
#endif

#endif //TANABATA_H
