#include "../core/core_func.h"
#include "../../include/tanabata.h"

uint64_t tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    return sasa_add(&tanabata->sasahyou, path);
}

int tanabata_sasa_rem(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_rem(&tanabata->sasahyou, sasa_id) == 0 &&
        kazari_rem_by_sasa(&tanabata->shoppyou, sasa_id) == 0) {
        return 0;
    }
    return 1;
}

int tanabata_sasa_upd(Tanabata *tanabata, uint64_t sasa_id, const char *path) {
    return sasa_upd(&tanabata->sasahyou, sasa_id, path);
}

Sasa tanabata_sasa_get(Tanabata *tanabata, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= tanabata->sasahyou.size) {
        return HOLE_SASA;
    }
    return tanabata->sasahyou.database[sasa_id];
}
