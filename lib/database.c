#include <malloc.h>
#include <string.h>
#include <sys/stat.h>

#include "../include/tanabata.h"

int tanabata_init(Tanabata *tanabata) {
    if (sasahyou_init(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_init(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_init(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_free(Tanabata *tanabata) {
    if (sasahyou_free(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_free(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_free(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_weed(Tanabata *tanabata) {
    uint64_t hole_cnt;
    uint64_t new_id;
    Kazari *current_kazari;
    if (tanabata->shoppyou.hole_cnt > 0) {
        hole_cnt = 0;
        current_kazari = tanabata->shoppyou.database;
        for (uint64_t i = 0; i < tanabata->shoppyou.size; i++) {
            if (current_kazari->sasa_id != HOLE_ID && current_kazari->tanzaku_id != HOLE_ID) {
                if (hole_cnt > 0) {
                    *(current_kazari - hole_cnt) = *current_kazari;
                }
            } else {
                hole_cnt++;
            }
            current_kazari++;
        }
        tanabata->shoppyou.size -= tanabata->shoppyou.hole_cnt;
        tanabata->shoppyou.hole_cnt = 0;
        free(tanabata->shoppyou.holes);
        tanabata->shoppyou.database = realloc(tanabata->shoppyou.database, tanabata->shoppyou.size * sizeof(Kazari));
    }
    if (tanabata->sasahyou.hole_cnt > 0) {
        hole_cnt = 0;
        Sasa *current_sasa = tanabata->sasahyou.database;
        for (uint64_t i = 0; i < tanabata->sasahyou.size; i++) {
            if (current_sasa->id != HOLE_ID) {
                if (hole_cnt > 0) {
                    new_id = current_sasa->id - hole_cnt;
                    current_kazari = tanabata->shoppyou.database;
                    for (uint64_t j = 0; j < tanabata->shoppyou.size; j++) {
                        if (current_kazari->sasa_id == current_sasa->id) {
                            current_kazari->sasa_id = new_id;
                        }
                        current_kazari++;
                    }
                    current_sasa->id = new_id;
                    *(current_sasa - hole_cnt) = *current_sasa;
                }
            } else {
                hole_cnt++;
            }
            current_sasa++;
        }
        tanabata->sasahyou.size -= tanabata->sasahyou.hole_cnt;
        tanabata->sasahyou.hole_cnt = 0;
        free(tanabata->sasahyou.holes);
        tanabata->sasahyou.database = realloc(tanabata->sasahyou.database, tanabata->sasahyou.size * sizeof(Sasa));
    }
    if (tanabata->sappyou.hole_cnt > 0) {
        hole_cnt = 0;
        Tanzaku *current_tanzaku = tanabata->sappyou.database;
        for (uint64_t i = 0; i < tanabata->sappyou.size; i++) {
            if (current_tanzaku->id != HOLE_ID) {
                if (hole_cnt > 0) {
                    new_id = current_tanzaku->id - hole_cnt;
                    current_kazari = tanabata->shoppyou.database;
                    for (uint64_t j = 0; j < tanabata->shoppyou.size; j++) {
                        if (current_kazari->tanzaku_id == current_tanzaku->id) {
                            current_kazari->tanzaku_id = new_id;
                        }
                        current_kazari++;
                    }
                    current_tanzaku->id = new_id;
                    *(current_tanzaku - hole_cnt) = *current_tanzaku;
                } else {
                    hole_cnt++;
                }
            }
            current_tanzaku++;
        }
        tanabata->sappyou.size -= tanabata->sappyou.hole_cnt;
        tanabata->sappyou.hole_cnt = 0;
        free(tanabata->sappyou.holes);
        tanabata->sappyou.database = realloc(tanabata->sappyou.database, tanabata->sappyou.size * sizeof(Tanzaku));
    }
    return 0;
}

int tanabata_load(Tanabata *tanabata) {
    if (sasahyou_load(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_load(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_load(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_save(Tanabata *tanabata) {
    if (sasahyou_save(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_save(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_save(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_open(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    if (sasahyou_open(&tanabata->sasahyou, strcat(file_path, "/sasahyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (sappyou_open(&tanabata->sappyou, strcat(file_path, "/sappyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (shoppyou_open(&tanabata->shoppyou, strcat(file_path, "/shoppyou")) != 0) {
        return 1;
    }
    free(file_path);
    return 0;
}

int tanabata_dump(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    if (sasahyou_dump(&tanabata->sasahyou, strcat(file_path, "/sasahyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (sappyou_dump(&tanabata->sappyou, strcat(file_path, "/sappyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (shoppyou_dump(&tanabata->shoppyou, strcat(file_path, "/shoppyou")) != 0) {
        return 1;
    }
    free(file_path);
    return 0;
}
