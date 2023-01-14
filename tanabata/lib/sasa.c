#include <stdlib.h>
#include <string.h>

#include "../core/core_func.h"
#include "../../include/tanabata.h"

int tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    if (path == NULL || tanabata->sasahyou.size == -1 && tanabata->sasahyou.hole_cnt == 0) {
        return 1;
    }
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath == NULL) {
        return 1;
    }
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, abspath) == 0) {
            return 1;
        }
        current_sasa++;
    }
    return sasa_add(&tanabata->sasahyou, abspath);
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
    if (tanabata->sasahyou.size == 0 || path == NULL) {
        return 1;
    }
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath == NULL) {
        return 1;
    }
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, abspath) == 0) {
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

int tanabata_sasa_upd(Tanabata *tanabata, uint64_t sasa_id, const char *path) {
    if (sasa_id == HOLE_ID || sasa_id >= tanabata->sasahyou.size) {
        return 1;
    }
    if (path == NULL) {
        return 0;
    }
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath == NULL) {
        return 1;
    }
    return sasa_upd(&tanabata->sasahyou, sasa_id, abspath);
}

Sasa tanabata_sasa_get_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= tanabata->sasahyou.size) {
        return HOLE_SASA;
    }
    return tanabata->sasahyou.database[sasa_id];
}

Sasa tanabata_sasa_get_by_path(Tanabata *tanabata, const char *path) {
    if (path == NULL) {
        return HOLE_SASA;
    }
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
