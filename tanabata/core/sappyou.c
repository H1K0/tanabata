#include <stdint.h>
#include <malloc.h>
#include <string.h>
#include <time.h>

#include "core_func.h"

const Tanzaku HOLE_TANZAKU = {HOLE_ID, 0, 0, NULL, NULL};

// Sappyou file signature: 七夕冊表
const uint16_t SAPPYOU_SIG[4] = {L'七', L'夕', L'冊', L'表'};

int sappyou_init(Sappyou *sappyou) {
    sappyou->created_ts = time(NULL);
    sappyou->modified_ts = sappyou->created_ts;
    sappyou->size = 0;
    sappyou->database = NULL;
    sappyou->hole_cnt = 0;
    sappyou->holes = NULL;
    sappyou->file = NULL;
    return 0;
}

int sappyou_free(Sappyou *sappyou) {
    sappyou->created_ts = 0;
    sappyou->modified_ts = 0;
    sappyou->size = 0;
    sappyou->hole_cnt = 0;
    if (sappyou->database != NULL) {
        for (Tanzaku *current_tanzaku = sappyou->database + sappyou->size - 1;
             current_tanzaku >= sappyou->database; current_tanzaku--) {
            free(current_tanzaku->name);
            free(current_tanzaku->description);
        }
        free(sappyou->database);
        sappyou->database = NULL;
    }
    free(sappyou->holes);
    sappyou->holes = NULL;
    if (sappyou->file != NULL) {
        fclose(sappyou->file);
        sappyou->file = NULL;
    }
    return 0;
}

int sappyou_load(Sappyou *sappyou) {
    if (sappyou->file == NULL ||
        (sappyou->file = freopen(NULL, "rb", sappyou->file)) == NULL) {
        return 1;
    }
    Sappyou temp;
    sappyou_init(&temp);
    temp.file = sappyou->file;
    uint16_t signature[4];
    if (fread(signature, 2, 4, temp.file) != 4 ||
        memcmp(signature, SAPPYOU_SIG, 8) != 0 ||
        fread(&temp.created_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.modified_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.size, 8, 1, temp.file) != 1 ||
        fread(&temp.hole_cnt, 8, 1, temp.file) != 1) {
        return 1;
    }
    temp.database = calloc(temp.size, sizeof(Tanzaku));
    temp.holes = calloc(temp.hole_cnt, sizeof(Tanzaku *));
    size_t max_string_len = SIZE_MAX;
    Tanzaku *current_tanzaku = temp.database;
    for (uint64_t i = 0, r = temp.hole_cnt; i < temp.size; i++, current_tanzaku++) {
        if (fgetc(temp.file) != 0) {
            current_tanzaku->id = i;
            if (fread(&current_tanzaku->created_ts, 8, 1, temp.file) != 1 ||
                fread(&current_tanzaku->modified_ts, 8, 1, temp.file) != 1 ||
                getdelim(&current_tanzaku->name, &max_string_len, 0, temp.file) == -1 ||
                getdelim(&current_tanzaku->description, &max_string_len, 0, temp.file) == -1) {
                temp.file = NULL;
                sappyou_free(&temp);
                return 1;
            }
        } else {
            current_tanzaku->id = HOLE_ID;
            if (r == 0) {
                temp.file = NULL;
                sappyou_free(&temp);
                return 1;
            }
            r--;
            temp.holes[r] = current_tanzaku;
        }
    }
    if (fflush(temp.file) == 0) {
        sappyou->file = NULL;
        sappyou_free(sappyou);
        *sappyou = temp;
        return 0;
    }
    temp.file = NULL;
    sappyou_free(&temp);
    return 1;
}

int sappyou_save(Sappyou *sappyou) {
    if (sappyou->file == NULL ||
        (sappyou->file = freopen(NULL, "wb", sappyou->file)) == NULL ||
        fwrite(SAPPYOU_SIG, 2, 4, sappyou->file) != 4 ||
        fwrite(&sappyou->created_ts, 8, 1, sappyou->file) != 1 ||
        fwrite(&sappyou->modified_ts, 8, 1, sappyou->file) != 1 ||
        fwrite(&sappyou->size, 8, 1, sappyou->file) != 1 ||
        fwrite(&sappyou->hole_cnt, 8, 1, sappyou->file) != 1 ||
        fflush(sappyou->file) != 0) {
        return 1;
    }
    Tanzaku *current_tanzaku = sappyou->database;
    for (uint64_t i = 0; i < sappyou->size; i++, current_tanzaku++) {
        if (current_tanzaku->id != HOLE_ID) {
            if (fputc(0xff, sappyou->file) == EOF ||
                fwrite(&current_tanzaku->created_ts, 8, 1, sappyou->file) != 1 ||
                fwrite(&current_tanzaku->modified_ts, 8, 1, sappyou->file) != 1 ||
                fputs(current_tanzaku->name, sappyou->file) == EOF ||
                fputc(0, sappyou->file) == EOF ||
                fputs(current_tanzaku->description, sappyou->file) == EOF ||
                fputc(0, sappyou->file) == EOF) {
                return 1;
            }
        } else if (fputc(0, sappyou->file) == EOF) {
            return 1;
        }
    }
    return fflush(sappyou->file);
}

int sappyou_open(Sappyou *sappyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (sappyou->file == NULL && (sappyou->file = fopen(path, "rb")) == NULL ||
        sappyou->file != NULL && (sappyou->file = freopen(path, "rb", sappyou->file)) == NULL) {
        return 1;
    }
    return sappyou_load(sappyou);
}

int sappyou_dump(Sappyou *sappyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (sappyou->file == NULL && (sappyou->file = fopen(path, "wb")) == NULL ||
        sappyou->file != NULL && (sappyou->file = freopen(path, "wb", sappyou->file)) == NULL) {
        return 1;
    }
    return sappyou_save(sappyou);
}

Tanzaku tanzaku_add(Sappyou *sappyou, const char *name, const char *description) {
    if (name == NULL || description == NULL || sappyou->size == -1 && sappyou->hole_cnt == 0) {
        return HOLE_TANZAKU;
    }
    Tanzaku newbie;
    newbie.created_ts = time(NULL);
    newbie.modified_ts = newbie.created_ts;
    newbie.name = malloc(strlen(name) + 1);
    strcpy(newbie.name, name);
    newbie.description = malloc(strlen(description) + 1);
    strcpy(newbie.description, description);
    if (sappyou->hole_cnt > 0) {
        sappyou->hole_cnt--;
        Tanzaku **hole_ptr = sappyou->holes + sappyou->hole_cnt;
        newbie.id = *hole_ptr - sappyou->database;
        **hole_ptr = newbie;
        sappyou->holes = reallocarray(sappyou->holes, sappyou->hole_cnt, sizeof(Tanzaku *));
    } else {
        newbie.id = sappyou->size;
        sappyou->size++;
        sappyou->database = reallocarray(sappyou->database, sappyou->size, sizeof(Tanzaku));
        sappyou->database[newbie.id] = newbie;
    }
    sappyou->modified_ts = newbie.created_ts;
    return newbie;
}

int tanzaku_rem(Sappyou *sappyou, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID || tanzaku_id >= sappyou->size) {
        return 1;
    }
    Tanzaku *current_tanzaku = sappyou->database + tanzaku_id;
    if (current_tanzaku->id == HOLE_ID) {
        return 1;
    }
    current_tanzaku->id = HOLE_ID;
    free(current_tanzaku->name);
    free(current_tanzaku->description);
    if (tanzaku_id == sappyou->size - 1) {
        sappyou->size--;
        sappyou->database = reallocarray(sappyou->database, sappyou->size, sizeof(Tanzaku));
    } else {
        sappyou->hole_cnt++;
        sappyou->holes = reallocarray(sappyou->holes, sappyou->hole_cnt, sizeof(Tanzaku *));
        sappyou->holes[sappyou->hole_cnt - 1] = current_tanzaku;
    }
    sappyou->modified_ts = time(NULL);
    return 0;
}

int tanzaku_upd(Sappyou *sappyou, uint64_t tanzaku_id, const char *name, const char *description) {
    if (tanzaku_id == HOLE_ID || tanzaku_id >= sappyou->size || name == NULL && description == NULL) {
        return 1;
    }
    Tanzaku *current_tanzaku = sappyou->database + tanzaku_id;
    if (current_tanzaku->id == HOLE_ID) {
        return 1;
    }
    if (name != NULL) {
        current_tanzaku->name = realloc(current_tanzaku->name, strlen(name) + 1);
        strcpy(current_tanzaku->name, name);
    }
    if (description != NULL) {
        current_tanzaku->description = realloc(current_tanzaku->description, strlen(description) + 1);
        strcpy(current_tanzaku->description, description);
    }
    sappyou->modified_ts = time(NULL);
    current_tanzaku->modified_ts = sappyou->modified_ts;
    return 0;
}
