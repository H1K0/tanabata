#include "../../include/tanabata.h"

int tanabata_sasa_add(Tanabata *tanabata, const char *path) {
    return sasa_add(&tanabata->sasahyou, path);
}

int tanabata_sasa_rem_by_id(Tanabata *tanabata, uint64_t sasa_id) {
    return sasa_rem_by_id(&tanabata->sasahyou, sasa_id);
}

int tanabata_sasa_rem_by_path(Tanabata *tanabata, const char *path) {
    return sasa_rem_by_path(&tanabata->sasahyou, path);
}
