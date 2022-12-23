#include <string.h>
#include "../../include/tanabata.h"

int tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *description) {
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        if (tanabata->sappyou.database[i].id != HOLE_ID && strcmp(tanabata->sappyou.database[i].name, name) == 0) {
            fprintf(stderr, "Failed to add tanzaku: target tanzaku is already added\n");
            return 1;
        }
    }
    return tanzaku_add(&tanabata->sappyou, name, description);
}

int tanabata_tanzaku_rem_by_id(Tanabata *tanabata, uint64_t tanzaku_id) {
    return tanzaku_rem(&tanabata->sappyou, tanzaku_id);
}

int tanabata_tanzaku_rem_by_name(Tanabata *tanabata, const char *name) {
    Tanzaku *current_tanzaku;
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        current_tanzaku = tanabata->sappyou.database + i;
        if (current_tanzaku->id != HOLE_ID && strcmp(current_tanzaku->name, name) == 0) {
            return tanzaku_rem(&tanabata->sappyou, current_tanzaku->id);
        }
    }
    fprintf(stderr, "Failed to remove tanzaku: target tanzaku does not exist\n");
    return 1;
}

Tanzaku tanabata_tanzaku_get_by_id(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        fprintf(stderr, "Failed to get tanzaku: got hole ID\n");
        return HOLE_TANZAKU;
    }
    if (tanzaku_id >= tanabata->sappyou.size) {
        fprintf(stderr, "Failed to get tanzaku: too big ID\n");
        return HOLE_TANZAKU;
    }
    return tanabata->sappyou.database[tanzaku_id];
}

Tanzaku tanabata_tanzaku_get_by_name(Tanabata *tanabata, const char *name) {
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
        if (strcmp(tanabata->sappyou.database[i].name, name) == 0) {
            return tanabata->sappyou.database[i];
        }
    }
    return HOLE_TANZAKU;
}
