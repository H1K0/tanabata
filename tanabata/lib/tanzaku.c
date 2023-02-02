#include "../core/core_func.h"
#include "../../include/tanabata.h"

Tanzaku tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *description) {
    return tanzaku_add(&tanabata->sappyou, name, description);
}

int tanabata_tanzaku_rem(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_rem(&tanabata->sappyou, tanzaku_id) == 0 &&
        kazari_rem_by_tanzaku(&tanabata->shoppyou, tanzaku_id) == 0) {
        return 0;
    }
    return 1;
}

int tanabata_tanzaku_upd(Tanabata *tanabata, uint64_t tanzaku_id, const char *name, const char *description) {
    return tanzaku_upd(&tanabata->sappyou, tanzaku_id, name, description);
}

Tanzaku tanabata_tanzaku_get(Tanabata *tanabata, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID || tanzaku_id >= tanabata->sappyou.size) {
        return HOLE_TANZAKU;
    }
    return tanabata->sappyou.database[tanzaku_id];
}
