// Tanabata file manager core lib
// By Masahiko AMANO aka H1K0

#ifndef TANABATA_CORE_H
#define TANABATA_CORE_H

#ifdef __cplusplus
#include <cstdint>
#include <cstdio>
extern "C" {
#else
#include <stdint.h>
#include <stdio.h>
#endif

// ==================== STRUCTS AND TYPEDEFS ==================== //

// Sasa (笹) - a file record
typedef struct sasa {
    uint64_t  id;           // Sasa ID
    uint64_t  created_ts;   // Sasa creation timestamp
    char     *path;         // File path
} Sasa;

// Tanzaku (短冊) - a tag record
typedef struct tanzaku {
    uint64_t  id;           // Tanzaku ID
    uint64_t  created_ts;   // Tanzaku creation timestamp
    uint64_t  modified_ts;  // Tanzaku last modification timestamp
    char     *name;         // Tanzaku name
    char     *alias;        // Tanzaku alias
    char     *description;  // Tanzaku description
} Tanzaku;

// Kazari (飾り) - a sasa-tanzaku association record
typedef struct kazari {
    uint64_t  created_ts;   // Kazari creation timestamp
    uint64_t  sasa_id;      // Sasa ID
    uint64_t  tanzaku_id;   // Tanzaku ID
} Kazari;

// Sasahyou (笹表) - database of files
typedef struct sasahyou {
    uint64_t  created_ts;   // Sasahyou creation timestamp
    uint64_t  modified_ts;  // Sasahyou last modification timestamp
    uint64_t  size;         // Sasahyou size (including unstaged units)
    uint64_t  removed_cnt;  // Number of removed sasa
    Sasa     *contents;     // Array of sasa
    FILE     *file;         // Storage file for sasahyou
} Sasahyou;

// Sappyou (冊表) - database of tanzaku
typedef struct sappyou {
    uint64_t  created_ts;   // Sappyou creation timestamp
    uint64_t  modified_ts;  // Sappyou last modification timestamp
    uint64_t  size;         // Sappyou size
    uint64_t  removed_cnt;  // Number of removed tanzaku
    Tanzaku  *contents;     // Array of tanzaku
    FILE     *file;         // Storage file for sappyou
} Sappyou;

// Shoppyou (飾表) - database of kazari
typedef struct shoppyou {
    uint64_t  created_ts;   // Shoppyou creation timestamp
    uint64_t  modified_ts;  // Shoppyou last modification timestamp
    uint64_t  size;         // Shoppyou size
    uint64_t  removed_cnt;  // Number of removed kazari
    Kazari   *contents;     // Array of kazari
    FILE     *file;         // Storage file for shoppyou
} Shoppyou;

// ==================== FILE SIGNATURES ==================== //

// Sasahyou file signature: 七夕笹表
static const uint16_t SASAHYOU_SIG[4] = {L'七', L'夕', L'笹', L'表'};
// Sappyou file signature: 七夕冊表
static const uint16_t SAPPYOU_SIG[4] = {L'七', L'夕', L'冊', L'表'};
// Shoppyou file signature: 七夕飾表
static const uint16_t SHOPPYOU_SIG[4] = {L'七', L'夕', L'飾', L'表'};

// ==================== SASAHYOU SECTION ==================== //

// Initialize empty sasahyou
int sasahyou_init(Sasahyou *sasahyou);

// Free sasahyou
int sasahyou_free(Sasahyou *sasahyou);

// Weed sasahyou
int sasahyou_weed(Sasahyou *sasahyou);

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

// Remove sasa from sasahyou by ID
int sasa_rem_by_id(Sasahyou *sasahyou, uint64_t sasa_id);

// Remove sasa from sasahyou by file path
int sasa_rem_by_path(Sasahyou *sasahyou, const char *path);

// ==================== SAPPYOU SECTION ==================== //

// Initialize empty sappyou
int sappyou_init(Sappyou *sappyou);

// Free sappyou
int sappyou_free(Sappyou *sappyou);

// Weed sappyou
int sappyou_weed(Sappyou *sappyou);

// Load sappyou from file
int sappyou_load(Sappyou *sappyou);

// Save sappyou to file
int sappyou_save(Sappyou *sappyou);

// Open sappyou file and load data from it
int sappyou_open(Sappyou *sappyou, const char *path);

// Dump sappyou to file
int sappyou_dump(Sappyou *sappyou, const char *path);

// Add new tanzaku to sappyou
int tanzaku_add(Sappyou *sappyou, const char *name, const char *alias, const char *description);

// Remove tanzaku from sappyou by ID
int tanzaku_rem_by_id(Sappyou *sappyou, uint64_t tanzaku_id);

// Remove tanzaku from sappyou by name
int tanzaku_rem_by_name(Sappyou *sappyou, const char *name);

// Remove tanzaku from sappyou by alias
int tanzaku_rem_by_alias(Sappyou *sappyou, const char *alias);

// ==================== SHOPPYOU SECTION ==================== //

// Initialize empty shoppyou
int shoppyou_init(Shoppyou *shoppyou);

// Free shoppyou
int shoppyou_free(Shoppyou *shoppyou);

// Weed shoppyou
int shoppyou_weed(Shoppyou *shoppyou);

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

#ifdef __cplusplus
}
#endif

#endif //TANABATA_CORE_H
