#include "../../include/tanabata.h"

int tanabata_kazari_add(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id) {
    return kazari_add(&tanabata->shoppyou, sasa_id, tanzaku_id);
}

int tanabata_kazari_rem(Tanabata *tanabata, uint64_t sasa_id, uint64_t tanzaku_id) {
    return kazari_rem(&tanabata->shoppyou, sasa_id, tanzaku_id);
}
