#include <malloc.h>
#include <string.h>
#include <time.h>
#include <stdio.h>

#include "../include/core.h"

const Sasa HOLE_SASA = {HOLE_ID};

// Sasahyou file signature: 七夕笹表
const uint16_t SASAHYOU_SIG[4] = {L'七', L'夕', L'笹', L'表'};

int sasahyou_init(Sasahyou *sasahyou) {
    sasahyou->created_ts = time(NULL);
    sasahyou->modified_ts = sasahyou->created_ts;
    sasahyou->size = 0;
    sasahyou->database = NULL;
    sasahyou->hole_cnt = 0;
    sasahyou->holes = NULL;
    sasahyou->file = NULL;
    return 0;
}

int sasahyou_free(Sasahyou *sasahyou) {
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        free(sasahyou->database[i].path);
    }
    free(sasahyou->database);
    free(sasahyou->holes);
    if (sasahyou->file != NULL) {
        fclose(sasahyou->file);
    }
    return 0;
}

int sasahyou_load(Sasahyou *sasahyou) {
    sasahyou->file = freopen(NULL, "rb", sasahyou->file);
    if (sasahyou->file == NULL) {
        return 1;
    }
    uint16_t signature[4];
    if (fread(signature, 2, 4, sasahyou->file) < 4 ||
        memcmp(signature, SASAHYOU_SIG, 8) != 0 ||
        fread(&sasahyou->created_ts, 8, 1, sasahyou->file) == 0 ||
        fread(&sasahyou->modified_ts, 8, 1, sasahyou->file) == 0 ||
        fread(&sasahyou->size, 8, 1, sasahyou->file) == 0 ||
        fread(&sasahyou->hole_cnt, 8, 1, sasahyou->file) == 0) {
        return 1;
    }
    sasahyou->database = malloc(sasahyou->size * sizeof(Sasa));
    sasahyou->holes = malloc(sasahyou->hole_cnt * sizeof(Sasa *));
    size_t max_path_len = SIZE_MAX;
    for (uint64_t i = 0, r = sasahyou->hole_cnt; i < sasahyou->size; i++) {
        if (fgetc(sasahyou->file) != 0) {
            sasahyou->database[i].id = i;
            if (fread(&sasahyou->database[i].created_ts, 8, 1, sasahyou->file) == 0 ||
                getdelim(&sasahyou->database[i].path, &max_path_len, 0, sasahyou->file) == -1) {
                return 1;
            }
        } else {
            sasahyou->database[i].id = HOLE_ID;
            r--;
            sasahyou->holes[r] = sasahyou->database + i;
        }
    }
    return fflush(sasahyou->file);
}

int sasahyou_save(Sasahyou *sasahyou) {
    sasahyou->file = freopen(NULL, "wb", sasahyou->file);
    if (sasahyou->file == NULL ||
        fwrite(SASAHYOU_SIG, 2, 4, sasahyou->file) < 4 ||
        fwrite(&sasahyou->created_ts, 8, 1, sasahyou->file) == 0 ||
        fwrite(&sasahyou->modified_ts, 8, 1, sasahyou->file) == 0 ||
        fwrite(&sasahyou->size, 8, 1, sasahyou->file) == 0 ||
        fwrite(&sasahyou->hole_cnt, 8, 1, sasahyou->file) == 0 ||
        fflush(sasahyou->file) != 0) {
        return 1;
    }
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (feof(sasahyou->file) != 0 || ferror(sasahyou->file) != 0) {
            return 1;
        }
        if (sasahyou->database[i].id != HOLE_ID) {
            if (fputc(-1, sasahyou->file) == EOF ||
                fwrite(&sasahyou->database[i].created_ts, 8, 1, sasahyou->file) == 0 ||
                fputs(sasahyou->database[i].path, sasahyou->file) == EOF ||
                fputc(0, sasahyou->file) == EOF) {
                return 1;
            }
        } else {
            if (fputc(0, sasahyou->file) == EOF) {
                return 1;
            }
        }
    }
    return fflush(sasahyou->file);
}

int sasahyou_open(Sasahyou *sasahyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    sasahyou->file = fopen(path, "rb");
    if (sasahyou->file == NULL) {
        return 1;
    }
    return sasahyou_load(sasahyou);
}

int sasahyou_dump(Sasahyou *sasahyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    sasahyou->file = fopen(path, "wb");
    if (sasahyou->file == NULL) {
        return 1;
    }
    return sasahyou_save(sasahyou);
}

int sasa_add(Sasahyou *sasahyou, const char *path) {
    if (path == NULL || sasahyou->size == -1 && sasahyou->hole_cnt == 0) {
        return 1;
    }
    Sasa newbie;
    newbie.created_ts = time(NULL);
    size_t path_size = strlen(path);
    newbie.path = malloc(path_size + 1);
    strcpy(newbie.path, path);
    if (sasahyou->hole_cnt > 0) {
        sasahyou->hole_cnt--;
        Sasa **hole_ptr = sasahyou->holes + sasahyou->hole_cnt;
        newbie.id = *hole_ptr - sasahyou->database;
        **hole_ptr = newbie;
        sasahyou->holes = realloc(sasahyou->holes, sasahyou->hole_cnt * sizeof(Sasa *));
    } else {
        newbie.id = sasahyou->size;
        sasahyou->size++;
        sasahyou->database = realloc(sasahyou->database, sasahyou->size * sizeof(Sasa));
        sasahyou->database[newbie.id] = newbie;
    }
    sasahyou->modified_ts = newbie.created_ts;
    return 0;
}

int sasa_rem(Sasahyou *sasahyou, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= sasahyou->size) {
        return 1;
    }
    if (sasahyou->database[sasa_id].id == HOLE_ID) {
        return 0;
    }
    sasahyou->database[sasa_id].id = HOLE_ID;
    if (sasa_id == sasahyou->size - 1) {
        sasahyou->size--;
        sasahyou->database = realloc(sasahyou->database, sasahyou->size * sizeof(Sasa));
    } else {
        sasahyou->hole_cnt++;
        sasahyou->holes = realloc(sasahyou->holes, sasahyou->hole_cnt * sizeof(Sasa *));
        sasahyou->holes[sasahyou->hole_cnt - 1] = sasahyou->database + sasa_id;
    }
    sasahyou->modified_ts = time(NULL);
    return 0;
}

int sasa_upd(Sasahyou *sasahyou, uint64_t sasa_id, const char *path) {
    if (sasa_id == HOLE_ID || sasa_id >= sasahyou->size) {
        return 1;
    }
    if (path != NULL) {
        strcpy(sasahyou->database[sasa_id].path, path);
        sasahyou->modified_ts = time(NULL);
    }
    return 0;
}
