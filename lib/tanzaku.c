#include <string.h>

#include "../include/tanabata.h"

int tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *description) {
    if (tanabata->sappyou.size == -1 && tanabata->sappyou.hole_cnt == 0) {
        return 1;
    }
    Tanzaku *current_tanzaku = tanabata->sappyou.database;
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        if (current_tanzaku->id != HOLE_ID && strcmp(current_tanzaku->name, name) == 0) {
            return 1;
        }
        current_tanzaku++;
    }
    return tanzaku_add(&tanabata->sappyou, name, description);
}

int tanabata_tanzaku_rem_by_id(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID || tanzaku_id >= tanabata->sappyou.size) {
        return 1;
    }
    if (tanzaku_rem(&tanabata->sappyou, tanzaku_id) == 0 &&
        kazari_rem_by_tanzaku(&tanabata->shoppyou, tanzaku_id) == 0) {
        return 0;
    }
    return 1;
}

int tanabata_tanzaku_rem_by_name(Tanabata *tanabata, const char *name) {
    Tanzaku *current_tanzaku = tanabata->sappyou.database;
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        if (current_tanzaku->id != HOLE_ID && strcmp(current_tanzaku->name, name) == 0) {
            if (tanzaku_rem(&tanabata->sappyou, current_tanzaku->id) == 0 &&
                kazari_rem_by_tanzaku(&tanabata->shoppyou, current_tanzaku->id) == 0) {
                return 0;
            }
            return 1;
        }
        current_tanzaku++;
    }
    return 1;
}

Tanzaku tanabata_tanzaku_get_by_id(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID || tanzaku_id >= tanabata->sappyou.size) {
        return HOLE_TANZAKU;
    }
    return tanabata->sappyou.database[tanzaku_id];
}

Tanzaku tanabata_tanzaku_get_by_name(Tanabata *tanabata, const char *name) {
    Tanzaku *current_tanzaku = tanabata->sappyou.database;
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        if (current_tanzaku->id != HOLE_ID && strcmp(current_tanzaku->name, name) == 0) {
            return tanabata->sappyou.database[i];
        }
        current_tanzaku++;
    }
    return HOLE_TANZAKU;
}
