#include <stdint.h>
#include <malloc.h>
#include <string.h>
#include <time.h>

#include "core_func.h"

const Sasa HOLE_SASA = {HOLE_ID, 0, NULL};

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
    sasahyou->created_ts = 0;
    sasahyou->modified_ts = 0;
    sasahyou->size = 0;
    sasahyou->hole_cnt = 0;
    if (sasahyou->database != NULL) {
        for (Sasa *current_sasa = sasahyou->database + sasahyou->size - 1;
             current_sasa >= sasahyou->database; current_sasa--) {
            free(current_sasa->path);
        }
        free(sasahyou->database);
        sasahyou->database = NULL;
    }
    free(sasahyou->holes);
    sasahyou->holes = NULL;
    if (sasahyou->file != NULL) {
        fclose(sasahyou->file);
        sasahyou->file = NULL;
    }
    return 0;
}

int sasahyou_load(Sasahyou *sasahyou) {
    if (sasahyou->file == NULL ||
        (sasahyou->file = freopen(NULL, "rb", sasahyou->file)) == NULL) {
        return 1;
    }
    Sasahyou temp;
    sasahyou_init(&temp);
    temp.file = sasahyou->file;
    uint16_t signature[4];
    if (fread(signature, 2, 4, temp.file) != 4 ||
        memcmp(signature, SASAHYOU_SIG, 8) != 0 ||
        fread(&temp.created_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.modified_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.size, 8, 1, temp.file) != 1 ||
        fread(&temp.hole_cnt, 8, 1, temp.file) != 1) {
        return 1;
    }
    temp.database = calloc(temp.size, sizeof(Sasa));
    temp.holes = calloc(temp.hole_cnt, sizeof(Sasa *));
    size_t max_path_len = SIZE_MAX;
    Sasa *current_sasa = temp.database;
    for (uint64_t i = 0, r = temp.hole_cnt; i < temp.size; i++) {
        if (fgetc(temp.file) != 0) {
            current_sasa->id = i;
            if (fread(&current_sasa->created_ts, 8, 1, temp.file) != 1 ||
                getdelim(&current_sasa->path, &max_path_len, 0, temp.file) == -1) {
                temp.file = NULL;
                sasahyou_free(&temp);
                return 1;
            }
        } else {
            current_sasa->id = HOLE_ID;
            if (r == 0) {
                temp.file = NULL;
                sasahyou_free(&temp);
                return 1;
            }
            r--;
            temp.holes[r] = current_sasa;
        }
        current_sasa++;
    }
    if (fflush(temp.file) == 0) {
        sasahyou->file = NULL;
        sasahyou_free(sasahyou);
        *sasahyou = temp;
        return 0;
    }
    temp.file = NULL;
    sasahyou_free(&temp);
    return 1;
}

int sasahyou_save(Sasahyou *sasahyou) {
    if (sasahyou->file == NULL ||
        (sasahyou->file = freopen(NULL, "wb", sasahyou->file)) == NULL ||
        fwrite(SASAHYOU_SIG, 2, 4, sasahyou->file) != 4 ||
        fwrite(&sasahyou->created_ts, 8, 1, sasahyou->file) != 1 ||
        fwrite(&sasahyou->modified_ts, 8, 1, sasahyou->file) != 1 ||
        fwrite(&sasahyou->size, 8, 1, sasahyou->file) != 1 ||
        fwrite(&sasahyou->hole_cnt, 8, 1, sasahyou->file) != 1 ||
        fflush(sasahyou->file) != 0) {
        return 1;
    }
    Sasa *current_sasa = sasahyou->database;
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (current_sasa->id != HOLE_ID) {
            if (fputc(0xff, sasahyou->file) == EOF ||
                fwrite(&current_sasa->created_ts, 8, 1, sasahyou->file) != 1 ||
                fputs(current_sasa->path, sasahyou->file) == EOF ||
                fputc(0, sasahyou->file) == EOF) {
                return 1;
            }
        } else if (fputc(0, sasahyou->file) == EOF) {
            return 1;
        }
        current_sasa++;
    }
    return fflush(sasahyou->file);
}

int sasahyou_open(Sasahyou *sasahyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (sasahyou->file == NULL && (sasahyou->file = fopen(path, "rb")) == NULL ||
        sasahyou->file != NULL && (sasahyou->file = freopen(path, "rb", sasahyou->file)) == NULL) {
        return 1;
    }
    return sasahyou_load(sasahyou);
}

int sasahyou_dump(Sasahyou *sasahyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (sasahyou->file == NULL && (sasahyou->file = fopen(path, "wb")) == NULL ||
        sasahyou->file != NULL && (sasahyou->file = freopen(path, "wb", sasahyou->file)) == NULL) {
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
    newbie.path = malloc(strlen(path) + 1);
    strcpy(newbie.path, path);
    if (sasahyou->hole_cnt > 0) {
        sasahyou->hole_cnt--;
        Sasa **hole_ptr = sasahyou->holes + sasahyou->hole_cnt;
        newbie.id = *hole_ptr - sasahyou->database;
        **hole_ptr = newbie;
        sasahyou->holes = reallocarray(sasahyou->holes, sasahyou->hole_cnt, sizeof(Sasa *));
    } else {
        newbie.id = sasahyou->size;
        sasahyou->size++;
        sasahyou->database = reallocarray(sasahyou->database, sasahyou->size, sizeof(Sasa));
        sasahyou->database[newbie.id] = newbie;
    }
    sasahyou->modified_ts = newbie.created_ts;
    return 0;
}

int sasa_rem(Sasahyou *sasahyou, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID || sasa_id >= sasahyou->size) {
        return 1;
    }
    Sasa *current_sasa = sasahyou->database + sasa_id;
    if (current_sasa->id == HOLE_ID) {
        return 1;
    }
    current_sasa->id = HOLE_ID;
    free(current_sasa->path);
    current_sasa->path = NULL;
    if (sasa_id == sasahyou->size - 1) {
        sasahyou->size--;
        sasahyou->database = reallocarray(sasahyou->database, sasahyou->size, sizeof(Sasa));
    } else {
        sasahyou->hole_cnt++;
        sasahyou->holes = reallocarray(sasahyou->holes, sasahyou->hole_cnt, sizeof(Sasa *));
        sasahyou->holes[sasahyou->hole_cnt - 1] = current_sasa;
    }
    sasahyou->modified_ts = time(NULL);
    return 0;
}

int sasa_upd(Sasahyou *sasahyou, uint64_t sasa_id, const char *path) {
    if (sasa_id == HOLE_ID || sasa_id >= sasahyou->size || path == NULL) {
        return 1;
    }
    Sasa *current_sasa = sasahyou->database + sasa_id;
    if (current_sasa->id == HOLE_ID) {
        return 1;
    }
    current_sasa->path = realloc(current_sasa->path, strlen(path) + 1);
    strcpy(current_sasa->path, path);
    sasahyou->modified_ts = time(NULL);
    return 0;
}
