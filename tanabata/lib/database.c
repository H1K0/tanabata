#include <malloc.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>

#include "../core/core_func.h"
#include "../../include/tanabata.h"

int tanabata_init(Tanabata *tanabata) {
    if (sasahyou_init(&tanabata->sasahyou) != 0 ||
        sappyou_init(&tanabata->sappyou) != 0 ||
        shoppyou_init(&tanabata->shoppyou) != 0) {
        return 1;
    }
    tanabata->sappyou.size = 1;
    tanabata->sappyou.database = malloc(sizeof(Tanzaku));
    tanabata->sappyou.database->id = 0;
    tanabata->sappyou.database->created_ts = tanabata->sappyou.created_ts;
    tanabata->sappyou.database->modified_ts = tanabata->sappyou.created_ts;
    tanabata->sappyou.database->name = malloc(9);
    tanabata->sappyou.database->description = malloc(30);
    strcpy(tanabata->sappyou.database->name, "FAVORITE");
    strcpy(tanabata->sappyou.database->description, "Special tanzaku for favorites");
    tanabata->sasahyou_mod = 0;
    tanabata->sappyou_mod = 0;
    tanabata->shoppyou_mod = 0;
    return 0;
}

int tanabata_free(Tanabata *tanabata) {
    if (sasahyou_free(&tanabata->sasahyou) != 0 ||
        sappyou_free(&tanabata->sappyou) != 0 ||
        shoppyou_free(&tanabata->shoppyou) != 0) {
        return 1;
    }
    tanabata->sasahyou_mod = 0;
    tanabata->sappyou_mod = 0;
    tanabata->shoppyou_mod = 0;
    return 0;
}

int tanabata_weed(Tanabata *tanabata) {
    uint64_t hole_cnt = 0, new_id;
    Kazari *current_kazari;
    Sasa *current_sasa = tanabata->sasahyou.database;
    for (uint64_t i = 0; i < tanabata->sasahyou.size; i++, current_sasa++) {
        if (current_sasa->id != HOLE_ID) {
            if (hole_cnt > 0) {
                new_id = current_sasa->id - hole_cnt;
                for (current_kazari = tanabata->shoppyou.database + tanabata->shoppyou.size - 1;
                     current_kazari >= tanabata->shoppyou.database; current_kazari--) {
                    if (current_kazari->sasa_id == current_sasa->id) {
                        current_kazari->sasa_id = new_id;
                    }
                }
                current_sasa->id = new_id;
                *(current_sasa - hole_cnt) = *current_sasa;
            }
        } else {
            kazari_rem_by_sasa(&tanabata->shoppyou, current_sasa->id);
            hole_cnt++;
        }
    }
    if (hole_cnt > 0) {
        tanabata->sasahyou.size -= hole_cnt;
        tanabata->sasahyou.hole_cnt = 0;
        free(tanabata->sasahyou.holes);
        tanabata->sasahyou.holes = NULL;
        tanabata->sasahyou.database = reallocarray(tanabata->sasahyou.database, tanabata->sasahyou.size,
                                                   sizeof(Sasa));
        tanabata->sasahyou.modified_ts = time(NULL);
    }
    hole_cnt = 0;
    Tanzaku *current_tanzaku = tanabata->sappyou.database;
    for (uint64_t i = 0; i < tanabata->sappyou.size; i++, current_tanzaku++) {
        if (current_tanzaku->id != HOLE_ID) {
            if (hole_cnt > 0) {
                new_id = current_tanzaku->id - hole_cnt;
                for (current_kazari = tanabata->shoppyou.database + tanabata->shoppyou.size - 1;
                     current_kazari >= tanabata->shoppyou.database; current_kazari--) {
                    if (current_kazari->tanzaku_id == current_tanzaku->id) {
                        current_kazari->tanzaku_id = new_id;
                    }
                }
                current_tanzaku->id = new_id;
                *(current_tanzaku - hole_cnt) = *current_tanzaku;
            } else {
                hole_cnt++;
            }
        }
    }
    if (hole_cnt > 0) {
        tanabata->sappyou.size -= tanabata->sappyou.hole_cnt;
        tanabata->sappyou.hole_cnt = 0;
        free(tanabata->sappyou.holes);
        tanabata->sappyou.holes = NULL;
        tanabata->sappyou.database = reallocarray(tanabata->sappyou.database, tanabata->sappyou.size,
                                                  sizeof(Tanzaku));
        tanabata->sappyou.modified_ts = time(NULL);
    }
    hole_cnt = 0;
    current_kazari = tanabata->shoppyou.database;
    for (uint64_t i = 0; i < tanabata->shoppyou.size; i++, current_kazari++) {
        if (current_kazari->sasa_id != HOLE_ID && current_kazari->tanzaku_id != HOLE_ID &&
            current_kazari->sasa_id < tanabata->sasahyou.size &&
            current_kazari->tanzaku_id < tanabata->sappyou.size) {
            if (hole_cnt > 0) {
                *(current_kazari - hole_cnt) = *current_kazari;
            }
        } else {
            hole_cnt++;
        }
    }
    if (hole_cnt > 0) {
        tanabata->shoppyou.size -= tanabata->shoppyou.hole_cnt;
        tanabata->shoppyou.hole_cnt = 0;
        free(tanabata->shoppyou.holes);
        tanabata->shoppyou.holes = NULL;
        tanabata->shoppyou.database = reallocarray(tanabata->shoppyou.database, tanabata->shoppyou.size,
                                                   sizeof(Kazari));
        tanabata->shoppyou.modified_ts = time(NULL);
    }
    return 0;
}

int tanabata_load(Tanabata *tanabata) {
    if (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts && sasahyou_load(&tanabata->sasahyou) != 0 ||
        tanabata->sappyou_mod != tanabata->sappyou.modified_ts && sappyou_load(&tanabata->sappyou) != 0 ||
        tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts && shoppyou_load(&tanabata->shoppyou) != 0) {
        return 1;
    }
    tanabata->sasahyou_mod = tanabata->sasahyou.modified_ts;
    tanabata->sappyou_mod = tanabata->sappyou.modified_ts;
    tanabata->shoppyou_mod = tanabata->shoppyou.modified_ts;
    return 0;
}

int tanabata_save(Tanabata *tanabata) {
    if (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts && sasahyou_save(&tanabata->sasahyou) != 0 ||
        tanabata->sappyou_mod != tanabata->sappyou.modified_ts && sappyou_save(&tanabata->sappyou) != 0 ||
        tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts && shoppyou_save(&tanabata->shoppyou) != 0) {
        return 1;
    }
    tanabata->sasahyou_mod = tanabata->sasahyou.modified_ts;
    tanabata->sappyou_mod = tanabata->sappyou.modified_ts;
    tanabata->shoppyou_mod = tanabata->shoppyou.modified_ts;
    return 0;
}

int tanabata_open(Tanabata *tanabata, const char *path) {
    if (path == NULL) {
        return 1;
    }
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    size_t pathlen = strlen(path);
    char *file_path = malloc(pathlen + 10);
    strcpy(file_path, path);
    if (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts &&
        sasahyou_open(&tanabata->sasahyou, strcpy(file_path + pathlen, "/sasahyou") - pathlen) != 0 ||
        tanabata->sappyou_mod != tanabata->sappyou.modified_ts &&
        sappyou_open(&tanabata->sappyou, strcpy(file_path + pathlen, "/sappyou") - pathlen) != 0 ||
        tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts &&
        shoppyou_open(&tanabata->shoppyou, strcpy(file_path + pathlen, "/shoppyou") - pathlen) != 0) {
        free(file_path);
        return 1;
    }
    free(file_path);
    tanabata->sasahyou_mod = tanabata->sasahyou.modified_ts;
    tanabata->sappyou_mod = tanabata->sappyou.modified_ts;
    tanabata->shoppyou_mod = tanabata->shoppyou.modified_ts;
    return 0;
}

int tanabata_dump(Tanabata *tanabata, const char *path) {
    if (path == NULL) {
        return 1;
    }
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    size_t pathlen = strlen(path);
    char *file_path = malloc(pathlen + 10);
    strcpy(file_path, path);
    if (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts &&
        sasahyou_dump(&tanabata->sasahyou, strcpy(file_path + pathlen, "/sasahyou") - pathlen) != 0 ||
        tanabata->sappyou_mod != tanabata->sappyou.modified_ts &&
        sappyou_dump(&tanabata->sappyou, strcpy(file_path + pathlen, "/sappyou") - pathlen) != 0 ||
        tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts &&
        shoppyou_dump(&tanabata->shoppyou, strcpy(file_path + pathlen, "/shoppyou") - pathlen) != 0) {
        free(file_path);
        return 1;
    }
    free(file_path);
    tanabata->sasahyou_mod = tanabata->sasahyou.modified_ts;
    tanabata->sappyou_mod = tanabata->sappyou.modified_ts;
    tanabata->shoppyou_mod = tanabata->shoppyou.modified_ts;
    return 0;
}
