#include <string.h>

#include "../../include/tanabata.h"

int tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
        if (tanabata->sasahyou.database[i].id != HOLE_ID && strcmp(tanabata->sasahyou.database[i].path, path) == 0) {
            fprintf(stderr, "Failed to add sasa: target file is already added\n");
            return 1;
        }
    }
    return sasa_add(&tanabata->sasahyou, path);
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
