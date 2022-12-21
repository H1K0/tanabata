#include "../../include/tanabata.h"

int tanabata_tanzaku_add(Tanabata *tanabata, const char *name, const char *alias, const char *description) {
    return tanzaku_add(&tanabata->sappyou, name, alias, description);
}

int tanabata_tanzaku_rem_by_id(Tanabata *tanabata, uint64_t tanzaku_id) {
    return tanzaku_rem_by_id(&tanabata->sappyou, tanzaku_id);
}

int tanabata_tanzaku_rem_by_name(Tanabata *tanabata, const char *name) {
    return tanzaku_rem_by_name(&tanabata->sappyou, name);
}

int tanabata_tanzaku_rem_by_alias(Tanabata *tanabata, const char *alias) {
    return tanzaku_rem_by_alias(&tanabata->sappyou, alias);
}
