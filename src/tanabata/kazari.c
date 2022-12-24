#include <malloc.h>

#include "../../include/tanabata.h"

int tanabata_kazari_add(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id >= tanabata->sasahyou.size) {
        return 1;
    }
    if (tanzaku_id >= tanabata->sappyou.size) {
        return 1;
    }
    Kazari *current_kazari = tanabata->shoppyou.database;
    for (uint64_t i = 0; i < tanabata->shoppyou.size; i++) {
        if (current_kazari->sasa_id == sasa_id && current_kazari->tanzaku_id == tanzaku_id) {
            return 0;
        }
        current_kazari++;
    }
    return kazari_add(&tanabata->shoppyou, sasa_id, tanzaku_id);
}

int tanabata_kazari_rem(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id) {
    return kazari_rem(&tanabata->shoppyou, sasa_id, tanzaku_id);
}

Tanzaku *tanabata_tanzaku_get_by_sasa(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID) {
        return NULL;
    }
    Tanzaku *tanzaku_list = NULL;
    uint64_t tanzaku_count = 0;
    Kazari *current_kazari = tanabata->shoppyou.database;
    for (uint64_t i = 0; i < tanabata->shoppyou.size; i++) {
        if (current_kazari->sasa_id == sasa_id) {
            tanzaku_count++;
            tanzaku_list = realloc(tanzaku_list, tanzaku_count * sizeof(Tanzaku));
            tanzaku_list[tanzaku_count - 1] = tanabata_tanzaku_get_by_id(tanabata, current_kazari->tanzaku_id);
        }
        current_kazari++;
    }
    if (tanzaku_list != NULL) {
        tanzaku_list = realloc(tanzaku_list, (tanzaku_count + 1) * sizeof(Tanzaku));
        tanzaku_list[tanzaku_count] = HOLE_TANZAKU;
    }
    return tanzaku_list;
}

Sasa *tanabata_sasa_get_by_tanzaku(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        return NULL;
    }
    Sasa *sasa_list = NULL;
    uint64_t sasa_count = 0;
    Kazari *current_kazari = tanabata->shoppyou.database;
    for (uint64_t i = 0; i < tanabata->shoppyou.size; i++) {
        if (current_kazari->tanzaku_id == tanzaku_id) {
            sasa_count++;
            sasa_list = realloc(sasa_list, sasa_count * sizeof(Sasa));
            sasa_list[sasa_count - 1] = tanabata_sasa_get_by_id(tanabata, current_kazari->sasa_id);
        }
        current_kazari++;
    }
    if (sasa_list != NULL) {
        sasa_list = realloc(sasa_list, (sasa_count + 1) * sizeof(Sasa));
        sasa_list[sasa_count] = HOLE_SASA;
    }
    return sasa_list;
}
