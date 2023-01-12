// Tanabata file manager core functions
// By Masahiko AMANO aka H1K0

#pragma once
#ifndef TANABATA_CORE_FUNC_H
#define TANABATA_CORE_FUNC_H

#ifdef __cplusplus
#include <cstdint>
extern "C" {
#else
#include <stdint.h>
#endif

#include "core.h"

// ==================== SASAHYOU SECTION ==================== //

// Initialize empty sasahyou
int sasahyou_init(Sasahyou *sasahyou);

// Free sasahyou
int sasahyou_free(Sasahyou *sasahyou);

// Load sasahyou from file
int sasahyou_load(Sasahyou *sasahyou);

// Save sasahyou to file
int sasahyou_save(Sasahyou *sasahyou);

// Open sasahyou file and load data from it
int sasahyou_open(Sasahyou *sasahyou, const char *path);

// Dump sasahyou to file
int sasahyou_dump(Sasahyou *sasahyou, const char *path);

// Add sasa to sasahyou
int sasa_add(Sasahyou *sasahyou, const char *path);

// Remove sasa from sasahyou
int sasa_rem(Sasahyou *sasahyou, uint64_t sasa_id);

// Update sasa file path
int sasa_upd(Sasahyou *sasahyou, uint64_t sasa_id, const char *path);

// ==================== SAPPYOU SECTION ==================== //

// Initialize empty sappyou
int sappyou_init(Sappyou *sappyou);

// Free sappyou
int sappyou_free(Sappyou *sappyou);

// Load sappyou from file
int sappyou_load(Sappyou *sappyou);

// Save sappyou to file
int sappyou_save(Sappyou *sappyou);

// Open sappyou file and load data from it
int sappyou_open(Sappyou *sappyou, const char *path);

// Dump sappyou to file
int sappyou_dump(Sappyou *sappyou, const char *path);

// Add new tanzaku to sappyou
int tanzaku_add(Sappyou *sappyou, const char *name, const char *description);

// Remove tanzaku from sappyou
int tanzaku_rem(Sappyou *sappyou, uint64_t tanzaku_id);

// Update tanzaku name and description
int tanzaku_upd(Sappyou *sappyou, uint64_t tanzaku_id, const char *name, const char *description);

// ==================== SHOPPYOU SECTION ==================== //

// Initialize empty shoppyou
int shoppyou_init(Shoppyou *shoppyou);

// Free shoppyou
int shoppyou_free(Shoppyou *shoppyou);

// Load shoppyou from file
int shoppyou_load(Shoppyou *shoppyou);

// Save shoppyou to file
int shoppyou_save(Shoppyou *shoppyou);

// Open shoppyou file and load data from it
int shoppyou_open(Shoppyou *shoppyou, const char *path);

// Dump shoppyou to file
int shoppyou_dump(Shoppyou *shoppyou, const char *path);

// Add kazari to shoppyou
int kazari_add(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id);

// Remove kazari from shoppyou
int kazari_rem(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id);

// Remove all kazari with a specific sasa ID from shoppyou
int kazari_rem_by_sasa(Shoppyou *shoppyou, uint64_t sasa_id);

// Remove all kazari with a specific tanzaku ID from shoppyou
int kazari_rem_by_tanzaku(Shoppyou *shoppyou, uint64_t tanzaku_id);

#ifdef __cplusplus
}
#endif

#endif //TANABATA_CORE_FUNC_H
