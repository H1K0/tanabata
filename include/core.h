// Tanabata file manager core names
// By Masahiko AMANO aka H1K0

#pragma once
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

// ==================== STRUCTS ==================== //

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

#ifdef __cplusplus
}
#endif

#endif //TANABATA_CORE_H
