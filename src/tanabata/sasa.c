#include <stdlib.h>
#include <string.h>

#include "../../include/tanabata.h"

int tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (tanabata->sasahyou.database[i].id != HOLE_ID && strcmp(tanabata->sasahyou.database[i].path, path) == 0) {
            fprintf(stderr, "Failed to add sasa: target file is already added\n");
            return 1;
        }
    }
    char *abspath = NULL;
    abspath = realpath(path, abspath);
    if (abspath != NULL) {
        return sasa_add(&tanabata->sasahyou, abspath);
    }
    fprintf(stderr, "Failed to add sasa: file does not exist\n");
    return 1;
}

int tanabata_sasa_rem_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    return sasa_rem(&tanabata->sasahyou, sasa_id);
}

int tanabata_sasa_rem_by_path(Tanabata *tanabata, const char *path) {
    Sasa *current_sasa;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        current_sasa = tanabata->sasahyou.database + i;
        if (current_sasa->id != HOLE_ID && strcmp(current_sasa->path, path) == 0) {
            return sasa_rem(&tanabata->sasahyou, current_sasa->id);
        }
    }
    fprintf(stderr, "Failed to remove sasa: target sasa does not exist\n");
    return 1;
}

Sasa tanabata_sasa_get_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID) {
        fprintf(stderr, "Failed to get sasa: got hole ID\n");
        return HOLE_SASA;
    }
    if (sasa_id >= tanabata->sasahyou.size) {
        fprintf(stderr, "Failed to get sasa: ID out of range\n");
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
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (tanabata->sasahyou.database[i].id != HOLE_ID && strcmp(tanabata->sasahyou.database[i].path, abspath) == 0) {
            return tanabata->sasahyou.database[i];
        }
    }
    return HOLE_SASA;
}
