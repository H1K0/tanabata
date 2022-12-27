#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../include/core.h"

const Tanzaku HOLE_TANZAKU = {HOLE_ID};

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
    for (uint64_t i = 0; i < sappyou->size; i++) {
        free(sappyou->database[i].name);
        free(sappyou->database[i].description);
    }
    free(sappyou->database);
    if (sappyou->file != NULL) {
        fclose(sappyou->file);
    }
    return 0;
}

int sappyou_load(Sappyou *sappyou) {
    sappyou->file = freopen(NULL, "rb", sappyou->file);
    if (sappyou->file == NULL) {
        return 1;
    }
    uint16_t signature[4];
    if (fread(signature, 2, 4, sappyou->file) < 4 ||
        memcmp(signature, SAPPYOU_SIG, 8) != 0 ||
        fread(&sappyou->created_ts, 8, 1, sappyou->file) == 0 ||
        fread(&sappyou->modified_ts, 8, 1, sappyou->file) == 0 ||
        fread(&sappyou->size, 8, 1, sappyou->file) == 0 ||
        fread(&sappyou->hole_cnt, 8, 1, sappyou->file) == 0) {
        return 1;
    }
    sappyou->database = malloc(sappyou->size * sizeof(Tanzaku));
    sappyou->holes = malloc(sappyou->hole_cnt * sizeof(Tanzaku *));
    size_t max_string_len = SIZE_MAX;
    for (uint64_t i = 0, r = sappyou->hole_cnt; i < sappyou->size; i++) {
        if (fgetc(sappyou->file) != 0) {
            sappyou->database[i].id = i;
            if (fread(&sappyou->database[i].created_ts, 8, 1, sappyou->file) == 0 ||
                fread(&sappyou->database[i].modified_ts, 8, 1, sappyou->file) == 0 ||
                getdelim(&sappyou->database[i].name, &max_string_len, 0, sappyou->file) == -1 ||
                getdelim(&sappyou->database[i].description, &max_string_len, 0, sappyou->file) == -1) {
                return 1;
            }
        } else {
            sappyou->database[i].id = HOLE_ID;
            r--;
            sappyou->holes[r] = sappyou->database + i;
        }
    }
    return fflush(sappyou->file);
}

int sappyou_save(Sappyou *sappyou) {
    sappyou->file = freopen(NULL, "wb", sappyou->file);
    if (sappyou->file == NULL ||
        fwrite(SAPPYOU_SIG, 2, 4, sappyou->file) < 4 ||
        fwrite(&sappyou->created_ts, 8, 1, sappyou->file) == 0 ||
        fwrite(&sappyou->modified_ts, 8, 1, sappyou->file) == 0 ||
        fwrite(&sappyou->size, 8, 1, sappyou->file) == 0 ||
        fwrite(&sappyou->hole_cnt, 8, 1, sappyou->file) == 0 ||
        fflush(sappyou->file) != 0) {
        return 1;
    }
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (sappyou->database[i].id != HOLE_ID) {
            if (fputc(-1, sappyou->file) == EOF ||
                fwrite(&sappyou->database[i].created_ts, 8, 1, sappyou->file) == 0 ||
                fwrite(&sappyou->database[i].modified_ts, 8, 1, sappyou->file) == 0 ||
                fputs(sappyou->database[i].name, sappyou->file) == EOF ||
                fputc(0, sappyou->file) == EOF ||
                fputs(sappyou->database[i].description, sappyou->file) == EOF ||
                fputc(0, sappyou->file) == EOF) {
                return 1;
            }
        } else {
            if (fputc(0, sappyou->file) == EOF) {
                return 1;
            }
        }
    }
    return fflush(sappyou->file);
}

int sappyou_open(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "rb");
    if (sappyou->file == NULL) {
        return 1;
    }
    return sappyou_load(sappyou);
}

int sappyou_dump(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "wb");
    if (sappyou->file == NULL) {
        return 1;
    }
    return sappyou_save(sappyou);
}

int tanzaku_add(Sappyou *sappyou, const char *name, const char *description) {
    if (sappyou->size == -1 && sappyou->hole_cnt == 0) {
        return 1;
    }
    Tanzaku newbie;
    newbie.created_ts = time(NULL);
    newbie.modified_ts = newbie.created_ts;
    size_t name_size = strlen(name),
            description_size = strlen(description);
    newbie.name = malloc(name_size + 1);
    strcpy(newbie.name, name);
    newbie.name[name_size] = 0;
    newbie.description = malloc(description_size + 1);
    strcpy(newbie.description, description);
    newbie.description[description_size] = 0;
    if (sappyou->hole_cnt > 0) {
        sappyou->hole_cnt--;
        Tanzaku **hole_ptr = sappyou->holes + sappyou->hole_cnt;
        newbie.id = *hole_ptr - sappyou->database;
        **hole_ptr = newbie;
        sappyou->holes = realloc(sappyou->holes, sappyou->hole_cnt * sizeof(Tanzaku *));
    } else {
        newbie.id = sappyou->size;
        sappyou->size++;
        sappyou->database = realloc(sappyou->database, sappyou->size * sizeof(Tanzaku));
        sappyou->database[newbie.id] = newbie;
    }
    sappyou->modified_ts = newbie.created_ts;
    return 0;
}

int tanzaku_rem(Sappyou *sappyou, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        return 1;
    }
    if (tanzaku_id >= sappyou->size) {
        return 1;
    }
    if (sappyou->database[tanzaku_id].id == HOLE_ID) {
        return 0;
    }
    sappyou->database[tanzaku_id].id = HOLE_ID;
    if (tanzaku_id == sappyou->size - 1) {
        sappyou->size--;
        sappyou->database = realloc(sappyou->database, sappyou->size * sizeof(Tanzaku));
    } else {
        sappyou->hole_cnt++;
        sappyou->holes = realloc(sappyou->holes, sappyou->hole_cnt);
        sappyou->holes[sappyou->hole_cnt - 1] = sappyou->database + tanzaku_id;
    }
    sappyou->modified_ts = time(NULL);
    return 0;
}
