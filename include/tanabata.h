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
    Sasahyou   sasahyou;   // Sasahyou struct
    Sappyou    sappyou;    // Sappyou struct
    Shoppyou   shoppyou;   // Shoppyou struct
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

// ==================== SASA SECTION ==================== //

// Add sasa
int tanabata_sasa_add(Tanabata *tanabata, const char *path);

// Remove sasa by ID
int tanabata_sasa_rem_by_id(Tanabata *tanabata, uint64_t sasa_id);

// Remove sasa by file path
int tanabata_sasa_rem_by_path(Tanabata *tanabata, const char *path);

// Get sasa by ID
Sasa tanabata_sasa_get_by_id(Tanabata *tanabata, uint64_t sasa_id);

// Get sasa by file path
Sasa tanabata_sasa_get_by_path(Tanabata *tanabata, const char *path);

// ==================== TANZAKU SECTION ==================== //

// Add tanzaku
int tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *description);

// Remove tanzaku by ID
int tanabata_tanzaku_rem_by_id(Tanabata *tanabata, uint64_t tanzaku_id);

// Remove tanzaku by name
int tanabata_tanzaku_rem_by_name(Tanabata *tanabata, const char *name);

// Get tanzaku by ID
Tanzaku tanabata_tanzaku_get_by_id(Tanabata *tanabata, uint64_t tanzaku_id);

// Get tanzaku by name
Tanzaku tanabata_tanzaku_get_by_name(Tanabata *tanabata, const char *name);

// ==================== KAZARI SECTION ==================== //

// Add kazari
int tanabata_kazari_add(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id);

// Remove kazari
int tanabata_kazari_rem(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id);

// Get tanzaku list of sasa
Tanzaku *tanabata_tanzaku_get_by_sasa(Tanabata *tanabata, uint64_t sasa_id);

// Get sasa list of tanzaku
Sasa *tanabata_sasa_get_by_tanzaku(Tanabata *tanabata, uint64_t tanzaku_id);

#ifdef __cplusplus
}
#endif

#endif //TANABATA_H
