// Tanabata lib
// By Masahiko AMANO aka H1K0

#pragma once
#ifndef TANABATA_H
#define TANABATA_H

#ifdef __cplusplus
#include <cstdint>
extern "C" {
#else
#include <stdint.h>
#endif

#include "core.h"

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
Sasa tanabata_sasa_add(Tanabata *tanabata, const char *path);

// Remove sasa by ID
int tanabata_sasa_rem(Tanabata *tanabata, uint64_t sasa_id);

// Update sasa file path
int tanabata_sasa_upd(Tanabata *tanabata, uint64_t sasa_id, const char *path);

// Get sasa by ID
Sasa tanabata_sasa_get(Tanabata *tanabata, uint64_t sasa_id);

// ==================== TANZAKU SECTION ==================== //

// Add tanzaku
Tanzaku tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *description);

// Remove tanzaku by ID
int tanabata_tanzaku_rem(Tanabata *tanabata, uint64_t tanzaku_id);

// Update tanzaku name and description
int tanabata_tanzaku_upd(Tanabata *tanabata, uint64_t tanzaku_id, const char *name, const char *description);

// Get tanzaku by ID
Tanzaku tanabata_tanzaku_get(Tanabata *tanabata, uint64_t tanzaku_id);

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
