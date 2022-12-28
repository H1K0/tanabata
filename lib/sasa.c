#include <stdlib.h>
#include <string.h>

#include "../include/tanabata.h"

int tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    if (tanabata->sasahyou.size == -1 && tanabata->sasahyou.hole_cnt == 0) {
        return 1;
    }
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, path) == 0) {
            return 1;
        }
        current_sasa++;
    }
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath != NULL) {
        return sasa_add(&tanabata->sasahyou, abspath);
    }
    return 1;
}

int tanabata_sasa_rem_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= tanabata->sasahyou.size) {
        return 1;
    }
    if (sasa_rem(&tanabata->sasahyou, sasa_id) == 0 &&
        kazari_rem_by_sasa(&tanabata->shoppyou, sasa_id) == 0) {
        return 0;
    }
    return 1;
}

int tanabata_sasa_rem_by_path(Tanabata *tanabata, const char *path) {
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, path) == 0) {
            if (sasa_rem(&tanabata->sasahyou, current_sasa->id) == 0 &&
                kazari_rem_by_sasa(&tanabata->shoppyou, current_sasa->id) == 0) {
                return 0;
            }
            return 1;
        }
        current_sasa++;
    }
    return 1;
}

Sasa tanabata_sasa_get_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= tanabata->sasahyou.size) {
        return HOLE_SASA;
    }
    return tanabata->sasahyou.database[sasa_id];
}

Sasa tanabata_sasa_get_by_path(Tanabata *tanabata, const char *path) {
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath == NULL) {
        return HOLE_SASA;
    }
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, abspath) == 0) {
            return *current_sasa;
        }
        current_sasa++;
    }
    return HOLE_SASA;
}
