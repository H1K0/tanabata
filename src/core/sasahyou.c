#include <malloc.h>
#include <string.h>
#include <time.h>
#include <stdio.h>

#include "../../include/core.h"

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
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to load sasahyou: input file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(sasahyou->file);
    fread(signature, 2, 4, sasahyou->file);
    if (memcmp(signature, SASAHYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load sasahyou: invalid signature\n");
        return 1;
    }
    fread(&sasahyou->created_ts, 8, 1, sasahyou->file);
    fread(&sasahyou->modified_ts, 8, 1, sasahyou->file);
    fread(&sasahyou->size, 8, 1, sasahyou->file);
    fread(&sasahyou->hole_cnt, 8, 1, sasahyou->file);
    sasahyou->database = malloc(sasahyou->size * sizeof(Sasa));
    sasahyou->holes = malloc(sasahyou->hole_cnt * sizeof(Sasa *));
    size_t max_path_len = SIZE_MAX;
    for (uint64_t i = 0, r = sasahyou->hole_cnt; i < sasahyou->size; i++) {
        if (fgetc(sasahyou->file) != 0) {
            sasahyou->database[i].id = i;
            fread(&sasahyou->database[i].created_ts, 8, 1, sasahyou->file);
            getdelim(&sasahyou->database[i].path, &max_path_len, 0, sasahyou->file);
        } else {
            sasahyou->database[i].id = HOLE_ID;
            r--;
            sasahyou->holes[r] = sasahyou->database + i;
        }
    }
    return 0;
}

int sasahyou_save(Sasahyou *sasahyou) {
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to save sasahyou: output file not specified\n");
        return 1;
    }
    rewind(sasahyou->file);
    fwrite(SASAHYOU_SIG, 2, 4, sasahyou->file);
    fwrite(&sasahyou->created_ts, 8, 1, sasahyou->file);
    fwrite(&sasahyou->modified_ts, 8, 1, sasahyou->file);
    fwrite(&sasahyou->size, 8, 1, sasahyou->file);
    fwrite(&sasahyou->hole_cnt, 8, 1, sasahyou->file);
    fflush(sasahyou->file);
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (sasahyou->database[i].id != HOLE_ID) {
            fputc(-1, sasahyou->file);
            fwrite(&sasahyou->database[i].created_ts, 8, 1, sasahyou->file);
            fputs(sasahyou->database[i].path, sasahyou->file);
            fputc(0, sasahyou->file);
        } else {
            fputc(0, sasahyou->file);
        }
    }
    fflush(sasahyou->file);
    return 0;
}

int sasahyou_open(Sasahyou *sasahyou, const char *path) {
    sasahyou->file = fopen(path, "r+b");
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to open sasahyou: failed to open file '%s'\n", path);
        return 1;
    }
    return sasahyou_load(sasahyou);
}

int sasahyou_dump(Sasahyou *sasahyou, const char *path) {
    sasahyou->file = fopen(path, "w+b");
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to dump sasahyou: failed to write to file '%s'\n", path);
        return 1;
    }
    return sasahyou_save(sasahyou);
}

int sasa_add(Sasahyou *sasahyou, const char *path) {
    if (sasahyou->size == -1 && sasahyou->hole_cnt == 0) {
        fprintf(stderr, "Failed to add sasa: sasahyou is full\n");
        return 1;
    }
    Sasa newbie;
    newbie.created_ts = time(NULL);
    size_t path_size = strlen(path);
    newbie.path = malloc(path_size + 1);
    strcpy(newbie.path, path);
    newbie.path[path_size] = 0;
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
    if (sasa_id == HOLE_ID) {
        fprintf(stderr, "Failed to remove sasa: got hole ID\n");
        return 1;
    }
    if (sasa_id >= sasahyou->size) {
        fprintf(stderr, "Failed to remove sasa: too big ID\n");
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
