// Tanabata DBMS core names
// By Masahiko AMANO aka H1K0

#pragma once
#ifndef TANABATA_DBMS_H
#define TANABATA_DBMS_H

#ifdef __cplusplus
extern "C" {
#endif

// TDBMS request codes
enum TRC {
    trc_db_stats = 0b0,
    trc_db_init = 0b11,
    trc_db_load = 0b10,
    trc_db_save = 0b100,
    trc_db_edit = 0b110,
    trc_db_remove_soft = 0b1,
    trc_db_remove_hard = 0b101,
    trc_db_weed = 0b111,
    trc_sasa_get = 0b10000,
    trc_sasa_get_by_tanzaku = 0b101000,
    trc_sasa_add = 0b10010,
    trc_sasa_update = 0b10100,
    trc_sasa_remove = 0b10001,
    trc_sasa_remove_by_tanzaku = 0b101001,
    trc_tanzaku_get = 0b100000,
    trc_tanzaku_get_by_sasa = 0b11000,
    trc_tanzaku_add = 0b100010,
    trc_tanzaku_update = 0b100100,
    trc_tanzaku_remove = 0b100001,
    trc_tanzaku_remove_by_sasa = 0b11001,
    trc_kazari_get = 0b1000,
    trc_kazari_add = 0b1010,
    trc_kazari_add_single_sasa_to_multiple_tanzaku = 0b11010,
    trc_kazari_add_single_tanzaku_to_multiple_sasa = 0b101010,
    trc_kazari_remove = 0b1001,
};

#ifdef __cplusplus
}
#endif

#endif //TANABATA_DBMS_H
