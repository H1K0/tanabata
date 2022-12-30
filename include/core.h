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
    uint64_t   id;           // Sasa ID
    uint64_t   created_ts;   // Sasa creation timestamp
    char      *path;         // File path
} Sasa;

// Tanzaku (短冊) - a tag record
typedef struct tanzaku {
    uint64_t   id;           // Tanzaku ID
    uint64_t   created_ts;   // Tanzaku creation timestamp
    uint64_t   modified_ts;  // Tanzaku last modification timestamp
    char      *name;         // Tanzaku name
    char      *description;  // Tanzaku description
} Tanzaku;

// Kazari (飾り) - a sasa-tanzaku association record
typedef struct kazari {
    uint64_t   sasa_id;      // Sasa ID
    uint64_t   tanzaku_id;   // Tanzaku ID
    uint64_t   created_ts;   // Kazari creation timestamp
} Kazari;

// Sasahyou (笹表) - database of sasa
typedef struct sasahyou {
    uint64_t   created_ts;   // Sasahyou creation timestamp
    uint64_t   modified_ts;  // Sasahyou last modification timestamp
    uint64_t   size;         // Sasahyou size (including holes)
    Sasa      *database;     // Array of sasa
    uint64_t   hole_cnt;     // Number of holes
    Sasa     **holes;        // Array of pointers to holes
    FILE      *file;         // Storage file for sasahyou
} Sasahyou;

// Sappyou (冊表) - database of tanzaku
typedef struct sappyou {
    uint64_t   created_ts;   // Sappyou creation timestamp
    uint64_t   modified_ts;  // Sappyou last modification timestamp
    uint64_t   size;         // Sappyou size (including holes)
    Tanzaku   *database;     // Array of tanzaku
    uint64_t   hole_cnt;     // Number of holes
    Tanzaku  **holes;        // Array of pointers to holes
    FILE      *file;         // Storage file for sappyou
} Sappyou;

// Shoppyou (飾表) - database of kazari
typedef struct shoppyou {
    uint64_t   created_ts;   // Shoppyou creation timestamp
    uint64_t   modified_ts;  // Shoppyou last modification timestamp
    uint64_t   size;         // Shoppyou size (including holes)
    Kazari    *database;     // Array of kazari
    uint64_t   hole_cnt;     // Number of holes
    Kazari   **holes;        // Array of pointers to holes
    FILE      *file;         // Storage file for shoppyou
} Shoppyou;

// ==================== CONSTANTS ==================== //

// ID of hole - an invalid record
#define HOLE_ID (-1)

// Hole sasa constant with hole ID
extern const Sasa HOLE_SASA;

// Hole tanzaku constant with hole ID
extern const Tanzaku HOLE_TANZAKU;

// Hole kazari constant with hole ID
extern const Kazari HOLE_KAZARI;

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

#endif //TANABATA_CORE_H
